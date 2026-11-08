package application

import (
	"context"
	"io"
	"sync"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// Control never waits for the consumer. Both event count and bytes are bounded.
type output struct {
	mu     sync.Mutex
	queue  []domain.GenerationEvent
	bytes  int
	ended  bool
	err    error
	wake   chan struct{}
	cancel func()
	once   sync.Once
}

func newOutput(cancel func()) *output { return &output{wake: make(chan struct{}, 1), cancel: cancel} }
func eventBytes(e domain.GenerationEvent) int {
	return len(e.Text) + len(e.FinishReason) + len(e.FailureCode) + len(e.Execution.ID) + len(e.Execution.Instance.ID) + 256
}
func (o *output) put(e domain.GenerationEvent) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := eventBytes(e)
	if o.ended || len(o.queue) >= 16 || o.bytes+n > 64*1024 {
		return false
	}
	o.queue = append(o.queue, e)
	o.bytes += n
	select {
	case o.wake <- struct{}{}:
	default:
	}
	return true
}
func (o *output) end(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.ended {
		o.ended = true
		o.err = err
	}
	select {
	case o.wake <- struct{}{}:
	default:
	}
}
func (o *output) Next(ctx context.Context) (domain.GenerationEvent, error) {
	for {
		o.mu.Lock()
		if len(o.queue) > 0 {
			e := o.queue[0]
			o.queue[0] = domain.GenerationEvent{}
			o.queue = o.queue[1:]
			o.bytes -= eventBytes(e)
			o.mu.Unlock()
			return e, nil
		}
		ended, err := o.ended, o.err
		o.mu.Unlock()
		if ended {
			if err == nil {
				err = io.EOF
			}
			return domain.GenerationEvent{}, err
		}
		select {
		case <-ctx.Done():
			o.Close()
			return domain.GenerationEvent{}, ctx.Err()
		case <-o.wake:
		}
	}
}
func (o *output) Close() error { o.once.Do(o.cancel); return nil }
