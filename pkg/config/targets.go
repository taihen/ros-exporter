package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

const defaultUsername = "prometheus"

// ErrUnknownTarget is returned when Resolve cannot find the target in the allowlist.
var ErrUnknownTarget = errors.New("unknown target")

// Target holds credentials for one RouterOS device.
type Target struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Port     string `json:"port"`
}

// File is the on-disk JSON shape for -config.file.
type File struct {
	DefaultUser string            `json:"default_user"`
	Targets     map[string]Target `json:"targets"`
}

// Store is an in-memory allowlist of target credentials.
type Store struct {
	defaultUser string
	targets     map[string]Target
}

// Load reads and validates a targets JSON file.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	defaultUser := strings.TrimSpace(f.DefaultUser)
	if defaultUser == "" {
		defaultUser = defaultUsername
	}

	if f.Targets == nil {
		f.Targets = map[string]Target{}
	}

	targets := make(map[string]Target, len(f.Targets))
	for name, t := range f.Targets {
		key := strings.TrimSpace(name)
		if key == "" {
			return nil, fmt.Errorf("config: empty target name")
		}
		if strings.TrimSpace(t.Password) == "" {
			return nil, fmt.Errorf("config: target %q: password is required", key)
		}
		targets[key] = Target{
			User:     strings.TrimSpace(t.User),
			Password: t.Password,
			Port:     strings.TrimSpace(t.Port),
		}
	}

	return &Store{
		defaultUser: defaultUser,
		targets:     targets,
	}, nil
}

// Resolve looks up target credentials. queryPort wins over the config port when set.
// The returned address is host, or host:port when a port is chosen.
func (s *Store) Resolve(target, queryPort string) (user, password, address string, err error) {
	if s == nil {
		return "", "", "", ErrUnknownTarget
	}

	t, ok := s.targets[target]
	if !ok {
		return "", "", "", fmt.Errorf("%w: %s", ErrUnknownTarget, target)
	}

	user = t.User
	if user == "" {
		user = s.defaultUser
	}
	password = t.Password

	port := strings.TrimSpace(queryPort)
	if port == "" {
		port = t.Port
	}

	address = target
	if port != "" {
		address = net.JoinHostPort(target, port)
	}
	return user, password, address, nil
}
