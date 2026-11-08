package llamacpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type run struct {
	backend    *Backend
	command    domain.ExecutionCommand
	ctx        context.Context
	cancel     context.CancelFunc
	events     chan domain.GenerationEvent
	released   chan struct{}
	evidence   domain.ReleaseEvidence
	releaseErr error
}

func (r *run) Reference() domain.ExecutionRef { return r.command.Execution }
func (r *run) Next(ctx context.Context) (domain.GenerationEvent, error) {
	select {
	case <-ctx.Done():
		return domain.GenerationEvent{}, ctx.Err()
	case e, ok := <-r.events:
		if !ok {
			return domain.GenerationEvent{}, io.EOF
		}
		return e, nil
	}
}
func (r *run) Cancel(context.Context) error { r.cancel(); return nil }
func (r *run) AwaitRelease(ctx context.Context) (domain.ReleaseEvidence, error) {
	select {
	case <-ctx.Done():
		return domain.ReleaseEvidence{}, ctx.Err()
	case <-r.released:
		return r.evidence, r.releaseErr
	}
}
func (r *run) emit(e domain.GenerationEvent) bool {
	e.Execution = r.Reference()
	select {
	case <-r.ctx.Done():
		return false
	case r.events <- e:
		return true
	}
}
func (r *run) post(ctx context.Context, path string, value any) (*http.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.backend.url(path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return r.backend.client.Do(req)
}
func (r *run) contextFits() (bool, error) {
	messages := make([]map[string]string, 0, len(r.command.Messages))
	for _, m := range r.command.Messages {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}
	resp, err := r.post(r.ctx, "/apply-template", map[string]any{"messages": messages, "add_generation_prompt": true})
	if err != nil {
		return false, err
	}
	var applied struct {
		Prompt string `json:"prompt"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&applied)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 {
		return false, errors.New("chat template unavailable")
	}
	resp, err = r.post(r.ctx, "/tokenize", map[string]any{"content": applied.Prompt, "add_special": false, "parse_special": true})
	if err != nil {
		return false, err
	}
	var tokens struct {
		Tokens []int `json:"tokens"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&tokens)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || tokens.Tokens == nil {
		return false, errors.New("tokenizer unavailable")
	}
	return len(tokens.Tokens)+r.command.MaxOutputTokens <= r.backend.contextTokens, nil
}
func (r *run) generate() {
	defer close(r.events)
	defer r.cancel()
	dispatched := false
	code := "upstream_error"
	err := func() error {
		fits, err := r.contextFits()
		if err != nil {
			return err
		}
		if !fits {
			code = "context_length_exceeded"
			return errors.New("context exceeded")
		}
		messages := make([]map[string]string, 0, len(r.command.Messages))
		for _, m := range r.command.Messages {
			messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
		}
		dispatched = true
		resp, err := r.post(r.ctx, "/v1/chat/completions", map[string]any{"model": r.command.Model.ID, "messages": messages, "max_tokens": r.command.MaxOutputTokens, "temperature": r.command.Temperature, "stream": true})
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			return errors.New("invalid backend response")
		}
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 128*1024)
		finish := ""
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				if finish == "" {
					return errors.New("missing finish reason")
				}
				r.emit(domain.GenerationEvent{Kind: domain.GenerationFinished, FinishReason: finish})
				return nil
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Finish *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) != 1 || finish != "" {
				return errors.New("invalid backend event")
			}
			choice := chunk.Choices[0]
			if choice.Delta.Content != "" {
				if !r.emit(domain.GenerationEvent{Kind: domain.GenerationDelta, Text: choice.Delta.Content}) {
					return r.ctx.Err()
				}
			}
			if choice.Finish != nil {
				if *choice.Finish != "stop" && *choice.Finish != "length" {
					return errors.New("unknown finish reason")
				}
				finish = *choice.Finish
			}
		}
		return errors.New("incomplete backend stream")
	}()
	if err != nil {
		r.emit(domain.GenerationEvent{Kind: domain.GenerationFailed, FailureCode: code})
	}
	// No dispatch is direct evidence; after any dispatch, cancellation and EOF are
	// insufficient. Observe the sole slot idle or let supervision stop the child.
	if !dispatched {
		r.confirm(domain.EvidenceExecutionCompleted)
		return
	}
	if err != nil {
		r.releaseErr = errors.New("ambiguous execution requires owned-process exit")
		close(r.released)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	timer := time.NewTicker(30 * time.Millisecond)
	defer timer.Stop()
	for {
		if r.backend.idle(ctx, r.backend.contextTokens) == nil {
			r.confirm(domain.EvidenceExecutionCompleted)
			return
		}
		select {
		case <-ctx.Done():
			r.releaseErr = ctx.Err()
			close(r.released)
			return
		case <-timer.C:
		}
	}
}
func (r *run) confirm(kind domain.EvidenceKind) {
	r.evidence = domain.ReleaseEvidence{Instance: r.Reference().Instance, Execution: r.Reference().ID, Kind: kind, ObservedAt: time.Now()}
	close(r.released)
}
