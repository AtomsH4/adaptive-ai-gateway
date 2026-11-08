package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AtomsH4/adaptive-ai-gateway/internal/adapter/config"
)

func TestStrictConfigRejectsUnknownFieldsRemoteListenAndInvalidBudgets(t *testing.T) {
	for _, text := range []string{"[server]\nlistenn='127.0.0.1:8787'", "[server]\nlisten='0.0.0.0:8787'", "[scheduler]\nqueue_timeout='0s'", "[backend]\nmode='cloud'"} {
		path := filepath.Join(t.TempDir(), "gateway.toml")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Load(path); err == nil {
			t.Fatalf("invalid configuration accepted: %s", text)
		}
	}
}
func TestParseDiagnosticsDoNotExposeConfigurationSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("unexpected_secret='private-value'"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatalf("unsafe parse result: %v", err)
	}
}
