package llamacpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

// The test binary can act as a real child HTTP process without model weights.
func TestMain(m *testing.M) {
	if os.Getenv("AIGATEWAY_TEST_CHILD") == "1" {
		helperServer()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func helperServer() {
	args := map[string]string{}
	for i := 1; i+1 < len(os.Args); i++ {
		if strings.HasPrefix(os.Args[i], "--") {
			args[os.Args[i]] = os.Args[i+1]
		}
	}
	key, _ := os.ReadFile(args["--api-key-file"])
	ctxSize, _ := strconv.Atoi(args["--ctx-size"])
	var processing atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/slots", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.TrimSpace(string(key)) {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode([]any{map[string]any{"n_ctx": ctxSize, "is_processing": processing.Load()}})
	})
	mux.HandleFunc("/apply-template", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"prompt":"test"}`) })
	mux.HandleFunc("/tokenize", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"tokens":[1,2]}`) })
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		processing.Store(true)
		defer processing.Store(false)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"child\"},\"finish_reason\":null}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			time.Sleep(250 * time.Millisecond)
			return
		case <-time.After(80 * time.Millisecond):
		}
		processing.Store(false)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	})
	_ = http.ListenAndServe("127.0.0.1:"+args["--port"], mux)
}
func testBackend(t *testing.T) (*Backend, domain.LoadCommand) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	path := t.TempDir() + "/fixture.gguf"
	contents := []byte("synthetic fixture, not model weights")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	exe, _ := os.Executable()
	model := domain.ModelRef{ID: "test", Revision: "1", ArtifactSHA256: hex.EncodeToString(digest[:])}
	profile := domain.RuntimeProfileRef{ID: "test", Revision: "1"}
	b, err := New(Settings{Executable: exe, ModelPath: path, Port: port, Threads: 1, Model: model, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	b.environment = append(b.environment, "AIGATEWAY_TEST_CHILD=1")
	cmd := domain.LoadCommand{Instance: domain.InstanceRef{ID: "test-owner", Epoch: 1}, Model: model, Profile: profile, ContextTokens: 2048, MaxConcurrency: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := b.Load(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := b.Stop(ctx, cmd.Instance); err != nil {
			t.Error(err)
		}
	})
	return b, cmd
}
func TestOwnedChildNormalCompletionAndConfirmedExit(t *testing.T) {
	b, cmd := testBackend(t)
	r, err := b.Begin(context.Background(), domain.ExecutionCommand{Execution: domain.ExecutionRef{Instance: cmd.Instance, ID: "run"}, Model: cmd.Model, Profile: cmd.Profile, MaxOutputTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	first, err := r.Next(ctx)
	if err != nil || first.Text != "child" {
		t.Fatalf("%+v %v", first, err)
	}
	last, err := r.Next(ctx)
	if err != nil || last.Kind != domain.GenerationFinished {
		t.Fatalf("%+v %v", last, err)
	}
	e, err := r.AwaitRelease(ctx)
	if err != nil || !e.ConfirmsExecution(r.Reference()) {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err := b.Stop(ctx, domain.InstanceRef{ID: "external", Epoch: 1}); err == nil {
		t.Fatal("foreign owner was accepted")
	}
	exit, err := b.Stop(ctx, cmd.Instance)
	if err != nil || exit.Kind != domain.EvidenceInstanceExited {
		t.Fatalf("%+v %v", exit, err)
	}
}
func TestHTTPDisconnectCannotProveCancelledComputeReleased(t *testing.T) {
	b, cmd := testBackend(t)
	r, err := b.Begin(context.Background(), domain.ExecutionCommand{Execution: domain.ExecutionRef{Instance: cmd.Instance, ID: "cancelled"}, Model: cmd.Model, Profile: cmd.Profile, MaxOutputTokens: 16})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	r.Cancel(ctx)
	e, err := r.AwaitRelease(ctx)
	if err == nil || e.ConfirmsExecution(r.Reference()) {
		t.Fatalf("disconnect released compute: %+v %v", e, err)
	}
	exit, err := b.Stop(ctx, cmd.Instance)
	if err != nil || !exit.ConfirmsExecution(r.Reference()) {
		t.Fatalf("exit did not release: %+v %v", exit, err)
	}
}
