package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/taihen/ros-exporter/pkg/config"
)

func TestResolveCredentialsFromStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	content := `{"targets":{"192.168.88.1":{"password":"secret","port":"8728"}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	user, password, address, ignored, err := resolveCredentials(
		store, true, "192.168.88.1", "ignored-user", "ignored-pass", "",
	)
	if err != nil {
		t.Fatalf("resolveCredentials: %v", err)
	}
	if !ignored {
		t.Fatal("expected query password to be marked ignored")
	}
	if user != "prometheus" || password != "secret" || address != "192.168.88.1:8728" {
		t.Fatalf("got user=%q password=%q address=%q", user, password, address)
	}
}

func TestResolveCredentialsUnknownTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	if err := os.WriteFile(path, []byte(`{"targets":{"192.168.88.1":{"password":"secret"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, _, err = resolveCredentials(store, false, "10.0.0.1", "", "", "")
	if !errors.Is(err, config.ErrUnknownTarget) {
		t.Fatalf("err=%v, want ErrUnknownTarget", err)
	}
}

func TestResolveCredentialsUnsafeQuery(t *testing.T) {
	user, password, address, ignored, err := resolveCredentials(
		nil, true, "192.168.88.1", "admin", "secret", "8729",
	)
	if err != nil {
		t.Fatalf("resolveCredentials: %v", err)
	}
	if ignored {
		t.Fatal("did not expect ignored password in unsafe mode")
	}
	if user != "admin" || password != "secret" || address != "192.168.88.1:8729" {
		t.Fatalf("got user=%q password=%q address=%q", user, password, address)
	}
}

func TestResolveCredentialsUnsafeDefaultUser(t *testing.T) {
	user, _, address, _, err := resolveCredentials(nil, true, "r1", "", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	if user != defaultUsername || address != "r1" {
		t.Fatalf("user=%q address=%q", user, address)
	}
}

func TestResolveCredentialsNoStoreNoUnsafe(t *testing.T) {
	_, _, _, _, err := resolveCredentials(nil, false, "r1", "", "x", "")
	if err == nil {
		t.Fatal("expected error when neither store nor unsafe auth is available")
	}
}
