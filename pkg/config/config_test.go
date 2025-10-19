package config

import (
	"os"
	"testing"
	"time"
)

func TestParse_DefaultsAndRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db?sslmode=disable")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("FLUSH_EVERY", "")
	t.Setenv("SHARDS", "")
	t.Setenv("MAX_CPU", "")
	t.Setenv("READ_MAX_RANGE_DAYS", "")
	t.Setenv("SHUTDOWN_WAIT", "")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":3000" {
		t.Fatalf("default LISTEN_ADDR expected :3000, got %q", cfg.ListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("default LOG_LEVEL expected info, got %q", cfg.LogLevel)
	}
	if cfg.FlushEvery != time.Second {
		t.Fatalf("default FLUSH_EVERY expected 1s, got %v", cfg.FlushEvery)
	}
	if cfg.Shards != 64 {
		t.Fatalf("default SHARDS expected 64, got %d", cfg.Shards)
	}
	if cfg.ReadMaxRangeDays != 90 {
		t.Fatalf("default READ_MAX_RANGE_DAYS expected 90, got %d", cfg.ReadMaxRangeDays)
	}
	if cfg.ShutdownWait != 5*time.Second {
		t.Fatalf("default SHUTDOWN_WAIT expected 5s, got %v", cfg.ShutdownWait)
	}
}

func TestParse_CustomValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db?sslmode=disable")
	t.Setenv("LISTEN_ADDR", ":8080")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("FLUSH_EVERY", "250ms")
	t.Setenv("SHARDS", "128")
	t.Setenv("MAX_CPU", "4")
	t.Setenv("READ_MAX_RANGE_DAYS", "7")
	t.Setenv("SHUTDOWN_WAIT", "2s")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.LogLevel != "debug" {
		t.Fatalf("custom addr/log not applied: %+v", cfg)
	}
	if cfg.FlushEvery != 250*time.Millisecond {
		t.Fatalf("expected 250ms, got %v", cfg.FlushEvery)
	}
	if cfg.Shards != 128 || cfg.MaxCPU != 4 || cfg.ReadMaxRangeDays != 7 || cfg.ShutdownWait != 2*time.Second {
		t.Fatalf("custom numeric/envs not applied: %+v", cfg)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name:    "missing DATABASE_URL",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name: "invalid SHARDS",
			env: map[string]string{
				"DATABASE_URL": "postgres://u:p@h:5432/db?sslmode=disable",
				"SHARDS":       "0",
			},
			wantErr: true,
		},
		{
			name: "negative READ_MAX_RANGE_DAYS",
			env: map[string]string{
				"DATABASE_URL":        "postgres://u:p@h:5432/db?sslmode=disable",
				"READ_MAX_RANGE_DAYS": "-1",
			},
			wantErr: true,
		},
		{
			name: "ok minimal",
			env: map[string]string{
				"DATABASE_URL": "postgres://u:p@h:5432/db?sslmode=disable",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// clear all related env
			for _, k := range []string{
				"DATABASE_URL", "LISTEN_ADDR", "LOG_LEVEL", "FLUSH_EVERY",
				"SHARDS", "MAX_CPU", "READ_MAX_RANGE_DAYS", "SHUTDOWN_WAIT",
			} {
				_ = os.Unsetenv(k)
			}
			// set case env
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Parse()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
