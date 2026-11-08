// Package llamacpp owns only the child process it started and its native protocol.
package llamacpp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
)

type Settings struct {
	Executable, ModelPath    string
	Port, GPULayers, Threads int
	Model                    domain.ModelRef
	Profile                  domain.RuntimeProfileRef
}
type Backend struct {
	mu            sync.Mutex
	cfg           Settings
	client        *http.Client
	ref           domain.InstanceRef
	owner         string
	retired       uint64
	cmd           *exec.Cmd
	exited        chan struct{}
	contextTokens int
	run           *run
	key           string
	environment   []string
	loading       chan struct{}
}

type authenticatedTransport struct {
	base http.RoundTripper
	key  string
}

func (t authenticatedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r := request.Clone(request.Context())
	r.Header.Set("Authorization", "Bearer "+t.key)
	return t.base.RoundTrip(r)
}

func New(cfg Settings) (*Backend, error) {
	if !filepathIsAbsolute(cfg.Executable) || !filepathIsAbsolute(cfg.ModelPath) || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("native backend requires absolute paths and a valid loopback port")
	}
	transport := &http.Transport{Proxy: nil, MaxIdleConns: 4, MaxConnsPerHost: 4, IdleConnTimeout: 30 * time.Second}
	key := rand.Text()
	environment := []string{}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE"} {
		if value, ok := os.LookupEnv(name); ok {
			environment = append(environment, name+"="+value)
		}
	}
	return &Backend{cfg: cfg, key: key, environment: environment, loading: make(chan struct{}, 1), client: &http.Client{Transport: authenticatedTransport{transport, key}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("backend redirects are forbidden") }}}, nil
}
func filepathIsAbsolute(p string) bool { return len(p) > 0 && p[0] == '/' }
func (b *Backend) url(path string) string {
	return "http://127.0.0.1:" + strconv.Itoa(b.cfg.Port) + path
}
func (b *Backend) Description() domain.BackendDescription {
	return domain.BackendDescription{Name: "llama.cpp", ProtocolVersion: "v0.9.0", Ownership: domain.OwnershipManaged, Capabilities: domain.BackendCapabilities{CanStopOwnedInstance: true, CanObserveExecutionEnd: true, CanValidateContextLimit: true}}
}

func (b *Backend) OwnedPIDs() []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd == nil {
		return nil
	}
	select {
	case <-b.exited:
		return nil
	default:
		return []int{b.cmd.Process.Pid}
	}
}

func (b *Backend) Load(ctx context.Context, command domain.LoadCommand) (domain.InstanceObservation, error) {
	if command.Instance.ID == "" || command.Instance.Epoch == 0 || command.ContextTokens < 512 {
		return domain.InstanceObservation{}, errors.New("invalid instance identity or context")
	}
	if command.Model != b.cfg.Model || command.Profile != b.cfg.Profile || command.MaxConcurrency != 1 {
		return domain.InstanceObservation{}, errors.New("unvalidated model or runtime profile")
	}
	select {
	case <-ctx.Done():
		return domain.InstanceObservation{}, ctx.Err()
	case b.loading <- struct{}{}:
	}
	defer func() { <-b.loading }()
	file, err := os.Open(b.cfg.ModelPath)
	if err != nil {
		return domain.InstanceObservation{}, errors.New("model is not readable")
	}
	h := sha256.New()
	var copyErr error
	buffer := make([]byte, 64*1024)
	for {
		if copyErr = ctx.Err(); copyErr != nil {
			break
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			_, _ = h.Write(buffer[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			copyErr = readErr
			break
		}
	}
	file.Close()
	if copyErr != nil || hex.EncodeToString(h.Sum(nil)) != b.cfg.Model.ArtifactSHA256 {
		return domain.InstanceObservation{}, errors.New("model digest mismatch")
	}
	b.mu.Lock()
	if err = ctx.Err(); err != nil {
		b.mu.Unlock()
		return domain.InstanceObservation{}, err
	}
	if b.ref.ID != "" || (b.owner != "" && b.owner != command.Instance.ID) || (b.owner == command.Instance.ID && command.Instance.Epoch <= b.retired) {
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("instance already exists or was retired")
	}
	// Do not attach to a pre-existing listener masquerading as our child.
	probe, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", b.cfg.Port))
	if listenErr != nil {
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("backend port is already in use")
	}
	probe.Close()
	keyFile, keyErr := os.CreateTemp("", "aigateway-backend-key-*")
	if keyErr != nil {
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("cannot prepare backend authentication")
	}
	keyPath := keyFile.Name()
	_, keyErr = keyFile.WriteString(b.key + "\n")
	keyFile.Close()
	if keyErr != nil {
		os.Remove(keyPath)
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("cannot prepare backend authentication")
	}
	cmd := exec.Command(b.cfg.Executable, "--model", b.cfg.ModelPath, "--ctx-size", strconv.Itoa(command.ContextTokens), "--parallel", "1", "--n-gpu-layers", strconv.Itoa(b.cfg.GPULayers), "--threads", strconv.Itoa(b.cfg.Threads), "--host", "127.0.0.1", "--port", strconv.Itoa(b.cfg.Port), "--alias", command.Model.ID, "--slots", "--no-webui", "--api-key-file", keyPath)
	cmd.Env = append([]string(nil), b.environment...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		os.Remove(keyPath)
		b.mu.Unlock()
		return domain.InstanceObservation{}, errors.New("cannot start managed backend")
	}
	exited := make(chan struct{})
	b.owner = command.Instance.ID
	b.ref = command.Instance
	b.cmd = cmd
	b.exited = exited
	b.contextTokens = command.ContextTokens
	b.mu.Unlock()
	go func() { _ = cmd.Wait(); os.Remove(keyPath); close(exited) }()
	timer := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return domain.InstanceObservation{}, ctx.Err()
		case <-exited:
			return domain.InstanceObservation{}, errors.New("managed backend exited while loading")
		case <-timer.C:
			if b.idle(ctx, command.ContextTokens) == nil {
				return domain.InstanceObservation{Instance: command.Instance, State: domain.InstanceReady}, nil
			}
		}
	}
}

// /slots provides compute-idle evidence, separate from HTTP EOF or cancellation.
func (b *Backend) idle(ctx context.Context, wantContext int) error {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, b.url("/slots"), nil)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("slots unavailable")
	}
	var slots []struct {
		Processing *bool `json:"is_processing"`
		Context    int   `json:"n_ctx"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&slots) != nil || len(slots) != 1 || slots[0].Processing == nil || *slots[0].Processing || slots[0].Context != wantContext {
		return errors.New("slot is busy or context differs")
	}
	return nil
}
func (b *Backend) InspectInstance(ctx context.Context, ref domain.InstanceRef) (domain.InstanceObservation, error) {
	b.mu.Lock()
	current, exited, capacity := b.ref, b.exited, b.contextTokens
	b.mu.Unlock()
	if current != ref {
		return domain.InstanceObservation{Instance: ref, State: domain.InstanceUnknown}, nil
	}
	select {
	case <-exited:
		e := domain.ReleaseEvidence{Instance: ref, Kind: domain.EvidenceInstanceExited, ObservedAt: time.Now()}
		return domain.InstanceObservation{Instance: ref, State: domain.InstanceExited, Evidence: &e}, nil
	default:
	}
	if err := b.idle(ctx, capacity); err != nil {
		return domain.InstanceObservation{Instance: ref, State: domain.InstanceUnknown}, err
	}
	return domain.InstanceObservation{Instance: ref, State: domain.InstanceReady}, nil
}
func (b *Backend) Stop(ctx context.Context, ref domain.InstanceRef) (domain.ReleaseEvidence, error) {
	if ref.ID == "" || ref.Epoch == 0 {
		return domain.ReleaseEvidence{}, errors.New("invalid instance identity")
	}
	b.mu.Lock()
	if b.owner == "" {
		b.owner = ref.ID
	}
	if b.owner != ref.ID {
		b.mu.Unlock()
		return domain.ReleaseEvidence{}, errors.New("instance is not owned")
	}
	if ref.Epoch > b.retired {
		b.retired = ref.Epoch
	}
	if b.ref != ref {
		b.mu.Unlock()
		return domain.ReleaseEvidence{Instance: ref, Kind: domain.EvidenceInstanceExited, ObservedAt: time.Now()}, nil
	}
	cmd, exited := b.cmd, b.exited
	b.mu.Unlock()
	select {
	case <-exited:
	default:
		_ = cmd.Process.Signal(os.Interrupt)
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-exited:
			timer.Stop()
		case <-timer.C:
			_ = cmd.Process.Kill()
		case <-ctx.Done():
			timer.Stop()
			_ = cmd.Process.Kill()
		}
		select {
		case <-exited:
		case <-ctx.Done():
			return domain.ReleaseEvidence{}, ctx.Err()
		}
	}
	b.mu.Lock()
	if b.ref == ref {
		b.ref = domain.InstanceRef{}
		b.cmd = nil
		b.run = nil
	}
	b.mu.Unlock()
	return domain.ReleaseEvidence{Instance: ref, Kind: domain.EvidenceInstanceExited, ObservedAt: time.Now()}, nil
}
func (b *Backend) Begin(ctx context.Context, command domain.ExecutionCommand) (ports.GenerationRun, error) {
	if command.Execution.ID == "" || command.Execution.Instance.ID == "" || command.Execution.Instance.Epoch == 0 {
		return nil, errors.New("invalid execution identity")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ref != command.Execution.Instance || command.Model != b.cfg.Model || command.Profile != b.cfg.Profile {
		return nil, errors.New("wrong model, profile or instance")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	r := &run{backend: b, command: command, ctx: runCtx, cancel: cancel, events: make(chan domain.GenerationEvent, 16), released: make(chan struct{})}
	b.run = r
	go r.generate()
	return r, nil
}
func (b *Backend) ObserveExecution(ctx context.Context, ref domain.ExecutionRef) (domain.ReleaseEvidence, error) {
	b.mu.Lock()
	r := b.run
	b.mu.Unlock()
	if r == nil || r.Reference() != ref {
		return domain.ReleaseEvidence{}, errors.New("unknown execution")
	}
	return r.AwaitRelease(ctx)
}
