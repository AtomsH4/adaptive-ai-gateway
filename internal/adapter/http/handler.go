// Package http maps the application use cases to the versioned HTTP contract.
package http

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type Handler struct {
	gateway      api.Gateway
	resources    api.ResourceAssessmentQuery
	model, token string
}

func New(gateway api.Gateway, resources api.ResourceAssessmentQuery, model, token string) http.Handler {
	return &Handler{gateway, resources, model, token}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := rand.Text()
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path != "/healthz" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+h.token)) != 1 {
		writeError(w, id, "authentication_failed", 401)
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/healthz":
		writeJSON(w, 200, map[string]string{"status": "ok"})
	case r.Method == "GET" && r.URL.Path == "/readyz":
		s, err := h.gateway.Status(r.Context())
		if err != nil || !s.ServiceReady {
			writeError(w, id, "service_not_ready", 503)
			return
		}
		writeJSON(w, 200, map[string]bool{"ready": true})
	case r.Method == "GET" && r.URL.Path == "/v1/models":
		writeJSON(w, 200, map[string]any{"object": "list", "data": []any{map[string]string{"id": h.model, "object": "model"}}})
	case r.Method == "GET" && r.URL.Path == "/admin/scheduler":
		s, err := h.gateway.Status(r.Context())
		if err != nil {
			writeError(w, id, "service_stopping", 503)
			return
		}
		history := make([]any, 0, len(s.RecentDecisions))
		for _, d := range s.RecentDecisions {
			history = append(history, map[string]any{"at": d.At, "code": d.Code})
		}
		writeJSON(w, 200, map[string]any{"recent_decisions": history, "service_ready": s.ServiceReady, "can_execute_now": s.CanExecuteNow, "can_queue": s.CanQueue, "engine_state": s.EngineState, "protection_state": s.ProtectionState, "active_executions": s.ActiveExecutions, "waiting_requests": s.WaitingRequests, "reason_codes": s.ReasonCodes})
	case r.Method == "GET" && r.URL.Path == "/admin/resources":
		h.resourceResponse(w, r, id)
	case r.Method == "POST" && r.URL.Path == "/v1/chat/completions":
		h.generate(w, r, id)
	default:
		writeError(w, id, "not_found", 404)
	}
}
func (h *Handler) resourceResponse(w http.ResponseWriter, r *http.Request, id string) {
	a, err := h.resources.Assess(r.Context())
	if err != nil {
		writeError(w, id, "telemetry_unavailable", 503)
		return
	}
	o := a.Snapshot.MemoryPressure
	quality := "unavailable"
	var value, at any
	if o.Quality == domain.Available {
		switch o.Value {
		case domain.PressureNormal:
			value = "normal"
		case domain.PressureWarning:
			value = "warning"
		case domain.PressureCritical:
			value = "critical"
		}
		if value != nil && !o.ObservedAt.IsZero() && o.Source != "" && !o.ObservedAt.After(a.AssessedAt) {
			quality = "available"
			at = o.ObservedAt
			if a.Signal.Reason == domain.ReasonTelemetryStale {
				quality = "stale"
			}
		} else {
			value = nil
		}
	}
	var free any
	if m := a.Snapshot.AvailableMemoryBytes; m.Quality == domain.Available && !m.ObservedAt.IsZero() && a.AssessedAt.Sub(m.ObservedAt) <= 6*time.Second {
		free = m.Value
	}
	cpu := map[string]any{"quality": "unavailable", "total_percent": nil, "owned_percent": nil, "other_percent": nil, "attribution_reliable": false}
	if observed := a.Snapshot.CPU; observed.Quality == domain.Available && !observed.ObservedAt.After(a.AssessedAt) && a.AssessedAt.Sub(observed.ObservedAt) <= 6*time.Second {
		cpu["quality"] = "available"
		cpu["total_percent"] = observed.Value.TotalPercent
		cpu["source"] = observed.Source
		if observed.Value.AttributionReliable {
			cpu["owned_percent"] = observed.Value.OwnedPercent
			cpu["other_percent"] = observed.Value.OtherPercent
			cpu["attribution_reliable"] = true
		}
	}
	writeJSON(w, 200, map[string]any{"assessed_at": a.AssessedAt, "memory_pressure": map[string]any{"value": value, "quality": quality, "observed_at": at, "source": o.Source}, "available_memory_bytes_estimate": free, "cpu": cpu, "gpu_utilization": map[string]any{"value": nil, "quality": "unavailable"}})
}

type message struct {
	Role    string  `json:"role"`
	Content *string `json:"content"`
}
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

func decodeRequest(data []byte) (chatRequest, string) {
	c := chatRequest{MaxTokens: 512, Temperature: 0.7}
	if !utf8.Valid(data) {
		return c, "invalid_request"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		if strings.HasPrefix(err.Error(), "json: unknown field") {
			return c, "unsupported_parameter"
		}
		return c, "invalid_request"
	}
	if decoder.Decode(new(any)) != io.EOF {
		return c, "invalid_request"
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil || raw == nil {
		return c, "invalid_request"
	}
	for _, value := range raw {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return c, "invalid_request"
		}
	}
	if c.Model == "" || len(c.Messages) == 0 || c.MaxTokens < 1 || c.MaxTokens > 512 || c.Temperature < 0 || c.Temperature > 2 {
		return c, "invalid_request"
	}
	for _, m := range c.Messages {
		if m.Content == nil || (m.Role != "system" && m.Role != "user" && m.Role != "assistant") {
			return c, "invalid_request"
		}
	}
	return c, ""
}
func (h *Handler) generate(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var size *http.MaxBytesError
		if errors.As(err, &size) {
			writeError(w, id, "request_too_large", 413)
		} else {
			writeError(w, id, "invalid_request", 400)
		}
		return
	}
	request, code := decodeRequest(data)
	if code != "" {
		writeError(w, id, code, 400)
		return
	}
	messages := make([]domain.Message, 0, len(request.Messages))
	for _, m := range request.Messages {
		messages = append(messages, domain.Message{Role: m.Role, Content: *m.Content})
	}
	out, err := h.gateway.Generate(r.Context(), api.GenerateRequest{ID: id, Model: request.Model, Messages: messages, MaxOutputTokens: request.MaxTokens, Temperature: request.Temperature})
	if err != nil {
		h.fail(w, id, err)
		return
	}
	defer out.Close()
	first, err := out.Next(r.Context())
	if err != nil {
		h.fail(w, id, err)
		return
	}
	if request.Stream {
		h.stream(w, r.Context(), id, out, first)
		return
	}
	var content strings.Builder
	event := first
	for {
		if event.Kind == domain.GenerationDelta {
			content.WriteString(event.Text)
		}
		if content.Len() > 1<<20 {
			h.fail(w, id, failureError("upstream_error"))
			return
		}
		if event.Kind == domain.GenerationFinished {
			writeJSON(w, 200, map[string]any{"id": id, "object": "chat.completion", "model": h.model, "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": content.String()}, "finish_reason": event.FinishReason}}})
			return
		}
		event, err = out.Next(r.Context())
		if err != nil {
			h.fail(w, id, err)
			return
		}
	}
}
func (h *Handler) stream(w http.ResponseWriter, ctx context.Context, id string, out api.Output, first domain.GenerationEvent) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	write := func(data any) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
		var payload []byte
		if text, ok := data.(string); ok {
			payload = []byte(text)
		} else {
			payload, _ = json.Marshal(data)
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	e := first
	for {
		delta := map[string]string{}
		var finish any
		if e.Kind == domain.GenerationDelta {
			delta["content"] = e.Text
		}
		if e.Kind == domain.GenerationFinished {
			finish = e.FinishReason
		}
		if !write(map[string]any{"id": id, "object": "chat.completion.chunk", "model": h.model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}) {
			return
		}
		if finish != nil {
			write("[DONE]")
			return
		}
		var err error
		e, err = out.Next(ctx)
		if err != nil {
			code := "upstream_error"
			var f *domain.Failure
			if errors.As(err, &f) {
				code = f.Code
			}
			write(errorBody(id, code))
			return
		}
	}
}
func failureError(code string) error { return &domain.Failure{Code: code} }
func (h *Handler) fail(w http.ResponseWriter, id string, err error) {
	code := "upstream_error"
	var f *domain.Failure
	if errors.As(err, &f) {
		code = f.Code
	}
	status := 502
	switch code {
	case "invalid_request", "unsupported_parameter", "context_length_exceeded":
		status = 400
	case "model_not_found":
		status = 404
	case "queue_full", "queue_timeout":
		status = 429
	case "telemetry_unavailable", "telemetry_stale", "memory_pressure", "memory_pressure_interrupted", "engine_unavailable", "model_load_failed", "service_stopping", "service_not_ready", "cpu_busy":
		status = 503
	case "model_load_timeout", "execution_timeout", "request_timeout":
		status = 504
	}
	writeError(w, id, code, status)
}
func errorBody(id, code string) any {
	return map[string]any{"error": map[string]any{"code": code, "message": "Request ended: " + code + ".", "type": "gateway_error", "param": nil}, "request_id": id}
}
func writeError(w http.ResponseWriter, id, code string, status int) {
	writeJSON(w, status, errorBody(id, code))
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
