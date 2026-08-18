package main

import "testing"

func TestLoadConfigUsesDefaultPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PORT", "")

	cfg, err := loadConfig()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("expected DatabaseURL to be %q but got %q", "postgres://example", cfg.DatabaseURL)
	}

	if cfg.Port != 8080 {
		t.Fatalf("expected Port to be %d, but got %d", 8080, cfg.Port)
	}
}

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "3000")

	_, err := loadConfig()

	if err == nil {
		t.Fatalf("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoadConfigUsesProvidedPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "something_")
	t.Setenv("PORT", "3000")

	cfg, err := loadConfig()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Port != 3000 {
		t.Fatalf("expected Port to be %d, but got %d", 3000, cfg.Port)
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port string
	}{
		{name: "non-numeric", port: "hello"},
		{name: "zero", port: "0"},
		{name: "out of range", port: "70000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://example")
			t.Setenv("PORT", tt.port)

			_, err := loadConfig()

			if err == nil {
				t.Fatalf("expected error for PORT %q, got nil", tt.port)
			}
		})
	}
}
