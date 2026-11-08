package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/config"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/fake"
	httpadapter "github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/http"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/llamacpp"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/observability"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/platform"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/application/ports"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/domain"
	"github.com/AtomsH4/adaptive-ai-gateway/internal/policy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func duration(s string) time.Duration { d, _ := time.ParseDuration(s); return d }
func run() error {
	path := flag.String("config", "configs/fake.toml", "TOML configuration path")
	check := flag.Bool("check-config", false, "validate configuration without starting a service")
	flag.Parse()
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	if *check {
		fmt.Println("configuration: valid")
		return nil
	}
	token, err := cfg.Token()
	if err != nil {
		return err
	}
	logger := observability.NewLogger(os.Stderr)
	model := domain.ModelRef{ID: cfg.Model.ID, Revision: cfg.Model.Revision, ArtifactSHA256: cfg.Model.SHA256}
	profile := domain.RuntimeProfileRef{ID: cfg.Backend.ProfileID, Revision: cfg.Backend.ProfileRevision}
	var backend ports.Backend
	var sampler ports.ResourceSampler
	if cfg.Backend.Mode == "fake" {
		backend = fake.NewBackend()
		sampler = fake.SyntheticSampler{}
	} else {
		backend, err = llamacpp.New(llamacpp.Settings{Executable: cfg.Backend.Executable, ModelPath: cfg.Model.Path, Port: cfg.Backend.Port, GPULayers: cfg.Backend.GPULayers, Threads: cfg.Backend.Threads, Model: model, Profile: profile})
		if err != nil {
			return err
		}
		sampler = &platform.HostSampler{Owned: backend.(ports.OwnedProcesses)}
	}
	collectCtx, stopCollect := context.WithCancel(context.Background())
	defer stopCollect()
	cache := &platform.Cache{}
	go cache.Collect(collectCtx, sampler, 2*time.Second)
	guard, _ := policy.NewResourceGuard(6 * time.Second)
	opts := application.Options{Model: model, Profile: profile, OwnerID: rand.Text(), ContextTokens: cfg.Model.ContextTokens, RequiredFreeBytes: uint64(cfg.Model.RequiredFreeMiB) << 20, QueueLimit: cfg.Scheduler.QueueLimit, QueueTimeout: duration(cfg.Scheduler.QueueTimeout), LoadTimeout: duration(cfg.Scheduler.LoadTimeout), ExecutionTimeout: duration(cfg.Scheduler.ExecutionTimeout), RequestTimeout: duration(cfg.Scheduler.RequestTimeout), IdleTimeout: duration(cfg.Scheduler.IdleTimeout), CriticalGrace: duration(cfg.Scheduler.CriticalGrace), CancelGrace: duration(cfg.Scheduler.CancelGrace), RecoveryWindow: duration(cfg.Scheduler.RecoveryWindow), ShutdownTimeout: duration(cfg.Scheduler.ShutdownTimeout)}
	coordinator, err := application.NewCoordinator(opts, backend, cache, policy.LoadPolicy{Memory: guard, OtherCPULimit: 85}, platform.Clock{})
	if err != nil {
		return err
	}
	assessment, _ := application.NewAssessmentService(cache, policy.LoadPolicy{Memory: guard, OtherCPULimit: 85}, platform.Clock{})
	listener, err := net.Listen("tcp", cfg.Server.Listen)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = coordinator.Close(ctx)
		return errors.New("cannot bind configured gateway address")
	}
	server := &http.Server{Handler: httpadapter.New(coordinator, assessment, cfg.Model.ID, token), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	logger.Info("gateway_started", "listen", listener.Addr().String(), "backend", cfg.Backend.Mode, "model", cfg.Model.ID)
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case <-signalCtx.Done():
	case err = <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http_server_stopped")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout+3*time.Second)
	defer cancel()
	// Close readiness and intake before waiting for active HTTP handlers.
	closeErr := coordinator.Close(ctx)
	_ = server.Shutdown(ctx)
	_ = server.Close()
	if closeErr != nil {
		return errors.New("shutdown incomplete: owned backend cleanup could not be confirmed")
	}
	logger.Info("gateway_stopped")
	return nil
}
