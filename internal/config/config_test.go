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
	t.Setenv("CRYPTO_CONFIG_FILE", "config.test")

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
	t.Setenv("CRYPTO_CONFIG_FILE", "")
	t.Setenv("HOME", t.TempDir())
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

func TestLoad_UserINIOverlaysProjectEnv(t *testing.T) {
	t.Setenv("CRYPTO_CONFIG_FILE", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "crypto-thing")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.ini"), []byte(`[coinbase]
api_key_name = organizations/new/apiKeys/new
api_private_key = new-private-key
`), 0600); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte(`DATABASE_URL=postgres://project/database
COINBASE_CLOUD_API_KEY_NAME=organizations/old/apiKeys/old
COINBASE_CLOUD_API_SECRET=old-private-key
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.URL != "postgres://project/database" {
		t.Fatalf("database URL = %q", cfg.Database.URL)
	}
	if cfg.Coinbase.APIKeyName != "organizations/new/apiKeys/new" || cfg.Coinbase.APIPrivateKey != "new-private-key" {
		t.Fatalf("user config was not preferred: key=%q private=%q", cfg.Coinbase.APIKeyName, cfg.Coinbase.APIPrivateKey)
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

func TestLoadINI_CoinbaseJWTKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	contents := `[database]
url = postgres://localhost/crypto

[coinbase]
api_key_name = organizations/example/apiKeys/example
api_private_key = escaped-private-key
rpm = 8
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Coinbase.APIKeyName != "organizations/example/apiKeys/example" {
		t.Fatalf("API key name = %q", cfg.Coinbase.APIKeyName)
	}
	if cfg.Coinbase.APIPrivateKey != "escaped-private-key" {
		t.Fatalf("private key = %q", cfg.Coinbase.APIPrivateKey)
	}
	if cfg.Coinbase.RPM != 8 {
		t.Fatalf("RPM = %d", cfg.Coinbase.RPM)
	}
}

func TestPreferredUserConfigPath_PrefersINI(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "crypto-thing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	iniPath := filepath.Join(dir, "config.ini")
	envPath := filepath.Join(dir, "config.env")
	if err := os.WriteFile(envPath, []byte("DATABASE_URL=env\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := preferredUserConfigPath(home); got != envPath {
		t.Fatalf("fallback path = %q, want %q", got, envPath)
	}
	if err := os.WriteFile(iniPath, []byte("[database]\nurl=ini\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := preferredUserConfigPath(home); got != iniPath {
		t.Fatalf("preferred path = %q, want %q", got, iniPath)
	}
}
