package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadAndResolve(t *testing.T) {
	path := writeTempConfig(t, `{
		"default_user": "prom",
		"targets": {
			"192.168.88.1": {
				"user": "admin",
				"password": "secret",
				"port": "8728"
			},
			"core-rtr.example.com": {
				"password": "other"
			}
		}
	}`)

	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	user, password, address, err := store.Resolve("192.168.88.1", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if user != "admin" || password != "secret" || address != "192.168.88.1:8728" {
		t.Fatalf("got user=%q password=%q address=%q", user, password, address)
	}

	user, password, address, err = store.Resolve("core-rtr.example.com", "")
	if err != nil {
		t.Fatalf("Resolve default user: %v", err)
	}
	if user != "prom" || password != "other" || address != "core-rtr.example.com" {
		t.Fatalf("got user=%q password=%q address=%q", user, password, address)
	}
}

func TestResolveQueryPortWins(t *testing.T) {
	path := writeTempConfig(t, `{
		"targets": {
			"192.168.88.1": {"password": "secret", "port": "8728"}
		}
	}`)
	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, _, address, err := store.Resolve("192.168.88.1", "8729")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if address != "192.168.88.1:8729" {
		t.Fatalf("address=%q, want host:8729", address)
	}
}

func TestResolveUnknownTarget(t *testing.T) {
	path := writeTempConfig(t, `{"targets": {"192.168.88.1": {"password": "secret"}}}`)
	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, _, _, err = store.Resolve("10.0.0.1", "")
	if !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("err=%v, want ErrUnknownTarget", err)
	}
}

func TestLoadMissingPassword(t *testing.T) {
	path := writeTempConfig(t, `{"targets": {"192.168.88.1": {"user": "admin"}}}`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := writeTempConfig(t, `{not json`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadDefaultUserFallback(t *testing.T) {
	path := writeTempConfig(t, `{"targets": {"r1": {"password": "x"}}}`)
	store, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	user, _, _, err := store.Resolve("r1", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if user != "prometheus" {
		t.Fatalf("user=%q, want prometheus", user)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected read error")
	}
}
