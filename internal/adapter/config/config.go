// Package config translates strict TOML and secret sources into validated values.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server    Server    `toml:"server"`
	Model     Model     `toml:"model"`
	Backend   Backend   `toml:"backend"`
	Scheduler Scheduler `toml:"scheduler"`
}
type Server struct {
	Listen   string `toml:"listen"`
	TokenEnv string `toml:"token_env"`
}
type Model struct {
	ID              string `toml:"id"`
	Revision        string `toml:"revision"`
	SHA256          string `toml:"sha256"`
	Path            string `toml:"path"`
	ContextTokens   int    `toml:"context_tokens"`
	RequiredFreeMiB int    `toml:"required_free_mib"`
}
type Backend struct {
	Mode            string `toml:"mode"`
	Executable      string `toml:"executable"`
	Port            int    `toml:"port"`
	GPULayers       int    `toml:"gpu_layers"`
	Threads         int    `toml:"threads"`
	ProfileID       string `toml:"profile_id"`
	ProfileRevision string `toml:"profile_revision"`
}
type Scheduler struct {
	QueueLimit       int    `toml:"queue_limit"`
	QueueTimeout     string `toml:"queue_timeout"`
	LoadTimeout      string `toml:"load_timeout"`
	ExecutionTimeout string `toml:"execution_timeout"`
	RequestTimeout   string `toml:"request_timeout"`
	IdleTimeout      string `toml:"idle_timeout"`
	CriticalGrace    string `toml:"critical_grace"`
	CancelGrace      string `toml:"cancel_grace"`
	RecoveryWindow   string `toml:"recovery_window"`
	ShutdownTimeout  string `toml:"shutdown_timeout"`
}

func Defaults() Config {
	return Config{
		Server:    Server{Listen: "127.0.0.1:8787", TokenEnv: "AIGATEWAY_TOKEN"},
		Model:     Model{ID: "validation-model", ContextTokens: 2048, RequiredFreeMiB: 2048},
		Backend:   Backend{Mode: "fake", Port: 8788, GPULayers: 99, Threads: 2, ProfileID: "local-default", ProfileRevision: "1"},
		Scheduler: Scheduler{QueueLimit: 4, QueueTimeout: "30s", LoadTimeout: "120s", ExecutionTimeout: "120s", RequestTimeout: "270s", IdleTimeout: "300s", CriticalGrace: "5s", CancelGrace: "3s", RecoveryWindow: "30s", ShutdownTimeout: "35s"},
	}
}

func Load(path string) (Config, error) {
	c := Defaults()
	path, err := filepath.Abs(path)
	if err != nil {
		return c, errors.New("invalid configuration path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read configuration: %w", err)
	}
	if len(data) > 65536 {
		return c, errors.New("configuration exceeds 64 KiB")
	}
	if err = toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&c); err != nil {
		// Parse diagnostics can contain input text. Keep secrets and arbitrary values private.
		var position *toml.DecodeError
		if errors.As(err, &position) {
			line, column := position.Position()
			return c, fmt.Errorf("invalid TOML or unknown field at line %d, column %d", line, column)
		}
		return c, errors.New("invalid TOML configuration or unknown field")
	}
	if c.Model.Path != "" && !filepath.IsAbs(c.Model.Path) {
		c.Model.Path = filepath.Join(filepath.Dir(path), c.Model.Path)
	}
	if c.Backend.Executable != "" && !filepath.IsAbs(c.Backend.Executable) {
		c.Backend.Executable = filepath.Join(filepath.Dir(path), c.Backend.Executable)
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Server.Listen)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("server.listen must use a loopback IP and a nonzero port")
	}
	if c.Server.TokenEnv == "" {
		return errors.New("server.token_env is required")
	}
	if c.Model.ID == "" || c.Model.ContextTokens < 512 || c.Model.RequiredFreeMiB < 1 {
		return errors.New("model ID, context >=512 and positive required_free_mib are required")
	}
	if c.Backend.Mode != "fake" && c.Backend.Mode != "llamacpp" {
		return errors.New("backend.mode must be fake or llamacpp")
	}
	if c.Backend.Port < 1 || c.Backend.Port > 65535 || c.Backend.Threads < 1 || c.Backend.GPULayers < 0 || c.Backend.ProfileID == "" || c.Backend.ProfileRevision == "" {
		return errors.New("invalid backend port, threads or profile")
	}
	if c.Backend.Mode == "llamacpp" && (c.Backend.Executable == "" || c.Model.Path == "" || len(c.Model.SHA256) != 64 || c.Model.Revision == "") {
		return errors.New("llamacpp requires executable, model path, revision and SHA-256")
	}
	if c.Scheduler.QueueLimit < 1 || c.Scheduler.QueueLimit > 4 {
		return errors.New("scheduler.queue_limit must be between 1 and 4")
	}
	for name, value := range map[string]string{"queue_timeout": c.Scheduler.QueueTimeout, "load_timeout": c.Scheduler.LoadTimeout, "execution_timeout": c.Scheduler.ExecutionTimeout, "request_timeout": c.Scheduler.RequestTimeout, "idle_timeout": c.Scheduler.IdleTimeout, "critical_grace": c.Scheduler.CriticalGrace, "cancel_grace": c.Scheduler.CancelGrace, "recovery_window": c.Scheduler.RecoveryWindow, "shutdown_timeout": c.Scheduler.ShutdownTimeout} {
		d, e := time.ParseDuration(value)
		if e != nil || d <= 0 || d > time.Hour {
			return fmt.Errorf("scheduler.%s must be a positive duration <=1h", name)
		}
	}
	return nil
}

func (c Config) Token() (string, error) {
	token := os.Getenv(c.Server.TokenEnv)
	if len(token) < 24 || strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("configured token environment variable must contain >=24 characters without whitespace")
	}
	return token, nil
}
