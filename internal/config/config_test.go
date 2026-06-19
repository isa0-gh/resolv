package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestValidateReportsAllConfigProblems(t *testing.T) {
	conf := &Config{
		Resolver:    "ftp://",
		TTL:         0,
		BindAddress: "not a socket",
		Hosts: map[string]string{
			"api.*.home": "127.0.0.1",
			"bad.local":  "not-an-ip",
		},
	}

	err := conf.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if len(validationErr.Problems) != 5 {
		t.Fatalf("expected 5 validation problems, got %d: %v", len(validationErr.Problems), validationErr.Problems)
	}

	message := err.Error()
	for _, want := range []string{
		"resolver",
		"ttl",
		"bind_address",
		"hosts.\"api.*.home\"",
		"hosts.\"bad.local\"",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected error to contain %q, got %q", want, message)
		}
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := []byte(`
resolver = "not-a-url"
ttl = -1
bind_address = ":0"

[hosts]
"*.home" = "127.0.0.1"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
}
