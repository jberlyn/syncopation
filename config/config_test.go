package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("DB_PATH", "test.db")
	os.Setenv("STORAGE_PATH", "test_storage")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("STORAGE_PATH")
	}()

	cfg := LoadConfig()

	if cfg.Port != "9999" {
		t.Errorf("Expected port 9999, got %s", cfg.Port)
	}
	if cfg.DBPath != "test.db" {
		t.Errorf("Expected DBPath test.db, got %s", cfg.DBPath)
	}
	if cfg.StoragePath != "test_storage" {
		t.Errorf("Expected StoragePath test_storage, got %s", cfg.StoragePath)
	}

	os.Unsetenv("PORT")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("STORAGE_PATH")

	cfgDefault := LoadConfig()
	if cfgDefault.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", cfgDefault.Port)
	}
	if cfgDefault.DBPath != "data/database/syncopation.sqlite" {
		t.Errorf("Expected default DBPath, got %s", cfgDefault.DBPath)
	}
	if cfgDefault.StoragePath != "data/resources" {
		t.Errorf("Expected default StoragePath, got %s", cfgDefault.StoragePath)
	}
}
