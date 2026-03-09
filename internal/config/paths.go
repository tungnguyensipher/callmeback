package config

import (
	"os"
	"path/filepath"
)

const (
	defaultAppDir = ".callmeback"
	defaultDBName = "callmeback.db"
)

func DatabasePath() (string, error) {
	if override := os.Getenv("CALLMEBACK_DB"); override != "" {
		return filepath.Clean(override), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultAppDir, defaultDBName), nil
}

func DatabaseDir() (string, error) {
	path, err := DatabasePath()
	if err != nil {
		return "", err
	}

	return filepath.Dir(path), nil
}
