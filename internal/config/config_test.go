package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvFromCurrentDirectory(t *testing.T) {
	// Create a temporary directory and .env file
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir) // Restore original directory

	// Change to temp directory
	os.Chdir(tempDir)

	// Set environment variable before loading .env file
	os.Setenv("CRYPTO_CONFIG_FILE", "config.test")

	envContent := `DATABASE_URL=postgres://test:test@localhost:5432/testdb?sslmode=disable
COINBASE_API_KEY=test_key
`

	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Create the referenced config file as .env format
	configContent := `COINBASE_API_SECRET=test_secret
COINBASE_RPM=5
`
	err = os.WriteFile("config.test", []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading config (should use .env from current directory)
	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the config values
	if cfg.Database.URL != "postgres://test:test@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("Expected DATABASE_URL to be 'postgres://test:test@localhost:5432/testdb?sslmode=disable', got '%s'", cfg.Database.URL)
	}

	if cfg.Coinbase.APIKey != "test_key" {
		t.Errorf("Expected COINBASE_API_KEY to be 'test_key', got '%s'", cfg.Coinbase.APIKey)
	}

	if cfg.Coinbase.APISecret != "test_secret" {
		t.Errorf("Expected COINBASE_API_SECRET to be 'test_secret', got '%s'", cfg.Coinbase.APISecret)
	}

	if cfg.Coinbase.RPM != 5 {
		t.Errorf("Expected COINBASE_RPM to be 5, got %d", cfg.Coinbase.RPM)
	}
}

func TestLoad_EnvFileWithDiscreteKeys(t *testing.T) {
	// Create a temporary directory and .env file with discrete DB keys
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir) // Restore original directory

	// Change to temp directory
	os.Chdir(tempDir)

	envContent := `DB_HOST=localhost
DB_PORT=5432
DB_NAME=testdb
DB_USER=testuser
DB_PASSWORD=testpass
DB_SSLMODE=require
COINBASE_API_KEY=test_key
`

	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Test loading config (no specific path, should use .env from current directory)
	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the database URL was constructed correctly
	expectedURL := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=require"
	if cfg.Database.URL != expectedURL {
		t.Errorf("Expected DATABASE_URL to be '%s', got '%s'", expectedURL, cfg.Database.URL)
	}

	if cfg.Coinbase.APIKey != "test_key" {
		t.Errorf("Expected COINBASE_API_KEY to be 'test_key', got '%s'", cfg.Coinbase.APIKey)
	}
}

func TestIsEnvFile(t *testing.T) {
	// Test .env file detection
	tempDir := t.TempDir()

	envContent := `KEY1=value1
KEY2=value2
# Comment line
KEY3=value3
`
	envFile := filepath.Join(tempDir, "test.env")
	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	if !isEnvFile(envFile) {
		t.Error("Expected isEnvFile to return true for .env file")
	}

	// Test .ini file detection
	iniContent := `[section1]
key1 = value1

[section2]
key2 = value2
`
	iniFile := filepath.Join(tempDir, "test.ini")
	err = os.WriteFile(iniFile, []byte(iniContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .ini file: %v", err)
	}

	if isEnvFile(iniFile) {
		t.Error("Expected isEnvFile to return false for .ini file")
	}
}
