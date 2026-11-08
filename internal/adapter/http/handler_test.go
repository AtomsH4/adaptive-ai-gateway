package http_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/fake"
	adapter "github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/http"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/api"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/policy"
)

type service struct{}

func (service) Generate(context.Context, api.GenerateRequest) (api.Output, error) {
	return &output{}, nil
}
func (service) Status(context.Context) (api.SchedulerStatus, error) {
	return api.SchedulerStatus{ServiceReady: true, EngineState: "unloaded", ProtectionState: "normal", CanQueue: true, ReasonCodes: []string{}}, nil
}

type output struct{ i int }

func (o *output) Next(context.Context) (domain.GenerationEvent, error) {
	o.i++
	if o.i == 1 {
		return domain.GenerationEvent{Kind: domain.GenerationDelta, Text: "你好"}, nil
	}
	if o.i == 2 {
		return domain.GenerationEvent{Kind: domain.GenerationFinished, FinishReason: "stop"}, nil
	}
	return domain.GenerationEvent{}, io.EOF
}
func (*output) Close() error { return nil }
func handler(t *testing.T) http.Handler {
	t.Helper()
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	assessment, _ := application.NewAssessmentService(fake.NewStaticSampler(domain.ResourceSnapshot{}), guard, fake.FixedClock{At: time.Now()})
	return adapter.New(service{}, assessment, "validation-model", "test-token-long-enough-for-demo")
}
func call(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHTTPAuthenticationStrictRequestAndReadiness(t *testing.T) {
	h := handler(t)
	for _, tc := range []struct {
		method, path, body, token string
		status                    int
	}{
		{"GET", "/healthz", "", "", 200},
		{"GET", "/readyz", "", "", 401},
		{"GET", "/readyz", "", "test-token-long-enough-for-demo", 200},
		{"POST", "/v1/chat/completions", `{"model":"validation-model","messages":[{"role":"user","content":"test"}],"tools":[]}`, "test-token-long-enough-for-demo", 400},
		{"POST", "/v1/chat/completions", `{"model":"validation-model","messages":[{"role":"user","content":null}]}`, "test-token-long-enough-for-demo", 400},
		{"POST", "/v1/chat/completions", `{"model":"validation-model","messages":[{"role":"user","content":"test"}],"temperature":null}`, "test-token-long-enough-for-demo", 400},
		{"POST", "/v1/chat/completions", strings.Repeat("x", (1<<20)+1), "test-token-long-enough-for-demo", 413},
	} {
		w := call(h, tc.method, tc.path, tc.body, tc.token)
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request identity")
		}
	}
}
func TestStreamUsesNormalFinishThenDone(t *testing.T) {
	w := call(handler(t), "POST", "/v1/chat/completions", `{"model":"validation-model","messages":[{"role":"user","content":"test"}],"stream":true}`, "test-token-long-enough-for-demo")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"finish_reason":"stop"`) || !strings.HasSuffix(w.Body.String(), "data: [DONE]\n\n") {
		t.Fatalf("invalid stream: %d %s", w.Code, w.Body.String())
	}
}
