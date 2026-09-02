package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Addr != ":8080" {
		t.Fatalf("expected addr :8080, got %q", cfg.Server.Addr)
	}
	if cfg.Workers != 4 {
		t.Fatalf("expected workers 4, got %d", cfg.Workers)
	}
	if cfg.QueueSize != 128 {
		t.Fatalf("expected queue size 128, got %d", cfg.QueueSize)
	}
	if cfg.Gateway.Timeout != "30s" {
		t.Fatalf("expected timeout 30s, got %q", cfg.Gateway.Timeout)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("expected data dir ./data, got %q", cfg.DataDir)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Fatalf("expected default addr, got %q", cfg.Server.Addr)
	}
	if cfg.Workers != 4 {
		t.Fatalf("expected default workers, got %d", cfg.Workers)
	}
}

func TestLoadValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
		"server": {"addr": ":9090"},
		"gateway": {"base_url": "http://gw", "timeout": "10s", "retries": 3},
		"workers": 8,
		"queue_size": 256,
		"data_dir": "/tmp/data"
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != ":9090" {
		t.Fatalf("expected addr :9090, got %q", cfg.Server.Addr)
	}
	if cfg.Gateway.BaseURL != "http://gw" {
		t.Fatalf("expected base url, got %q", cfg.Gateway.BaseURL)
	}
	if cfg.Gateway.Retries != 3 {
		t.Fatalf("expected retries 3, got %d", cfg.Gateway.Retries)
	}
	if cfg.Workers != 8 {
		t.Fatalf("expected workers 8, got %d", cfg.Workers)
	}
	if cfg.QueueSize != 256 {
		t.Fatalf("expected queue size 256, got %d", cfg.QueueSize)
	}
	if cfg.DataDir != "/tmp/data" {
		t.Fatalf("expected data dir /tmp/data, got %q", cfg.DataDir)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGatewayTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		want    time.Duration
	}{
		{"empty falls back", "", 30 * time.Second},
		{"valid", "10s", 10 * time.Second},
		{"invalid falls back", "not-a-duration", 30 * time.Second},
		{"milliseconds", "1500ms", 1500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Gateway: GatewayConfig{Timeout: tt.timeout}}
			if got := cfg.GatewayTimeout(); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
