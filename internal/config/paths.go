package config

import (
	"os"
	"path/filepath"
)

const (
	defaultBaseDir = ".openharness"
	configFileName = "settings.json"
)

func ensureDir(path string) (string, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// ConfigDir returns the config dir with OPENHARNESS_CONFIG_DIR override support.
func ConfigDir() (string, error) {
	if env := os.Getenv("OPENHARNESS_CONFIG_DIR"); env != "" {
		return ensureDir(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(home, defaultBaseDir))
}

// ConfigFilePath returns ~/.openharness/settings.json or env override path.
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// DataDir returns ~/.openharness/data or OPENHARNESS_DATA_DIR.
func DataDir() (string, error) {
	if env := os.Getenv("OPENHARNESS_DATA_DIR"); env != "" {
		return ensureDir(env)
	}
	cfg, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(cfg, "data"))
}

// SessionsDir returns session storage directory.
func SessionsDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(dataDir, "sessions"))
}
