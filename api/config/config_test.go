// path: api/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	_ = os.Unsetenv("DATABASE_URL")
	_ = os.Unsetenv("PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.DBDSN == "" {
		t.Fatal("expected non-empty default database DSN")
	}
}

func TestConfigLoadCustom(t *testing.T) {
	testDSN := "postgres://test_user:test_pass@localhost:5432/test_db?sslmode=disable"
	_ = os.Setenv("DATABASE_URL", testDSN)
	_ = os.Setenv("PORT", "9090")
	defer func() {
		_ = os.Unsetenv("DATABASE_URL")
		_ = os.Unsetenv("PORT")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load custom config: %v", err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("expected custom port 9090, got %s", cfg.Port)
	}
	if cfg.DBDSN != testDSN {
		t.Fatalf("expected custom DSN %s, got %s", testDSN, cfg.DBDSN)
	}
}

