package config

import (
	"context"
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
	"net/url"
	"strconv"
	"strings"

	ini "gopkg.in/ini.v1"
	"github.com/joho/godotenv"
)

// Context plumb for config

type contextKey struct{}

func WithConfig(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

func FromContext(ctx context.Context) *Config {
	if v := ctx.Value(contextKey{}); v != nil {
		if c, ok := v.(*Config); ok {
			return c
		}
	}
	return &Config{}
}

type Config struct {
	Database struct {
		URL string
	}
	Coinbase struct {
		APIKey     string
		APISecret  string
		Passphrase string
		APIKeyName    string
		APIPrivateKey string
		RPM         int
		MaxRetries  int
		BackoffMS   int
	}
	App struct {
		Verbose bool
	}
}

// CoinbaseCreds represents the structure of the Coinbase credentials JSON file.
type CoinbaseCreds struct {
	Name       string `json:"name"`
	PrivateKey string `json:"privateKey"`
}

func Load(path, credsPath string) (*Config, error) {
	// If no specific path is provided, implement the new .env-based loading logic
	if path == "" {
		// First, try to load .env file from current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}

		envFile := filepath.Join(cwd, ".env")
		if _, err := os.Stat(envFile); err == nil {
			// Load environment variables from .env file in current directory
			if err := godotenv.Load(envFile); err != nil {
				return nil, fmt.Errorf("load env file: %w", err)
			}

			// Check for CRYPTO_CONFIG_FILE variable
			if configFile := os.Getenv("CRYPTO_CONFIG_FILE"); configFile != "" {
				path = configFile
			}
		} else {
			// Fall back to old logic if no .env file in current directory
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			// Try .env first, then fall back to .ini
			envPath := filepath.Join(home, ".config", "crypto-thing", "config.env")
			iniPath := filepath.Join(home, ".config", "crypto-thing", "config.ini")

			// Check if .env file exists
			if _, err := os.Stat(envPath); err == nil {
				path = envPath
			} else if _, err := os.Stat(iniPath); err == nil {
				path = iniPath
			} else {
				// Neither file exists, default to .env path
				path = envPath
			}
		}
	}

	var c Config

	// Check if file is .env or .ini based on extension or content
	if strings.HasSuffix(strings.ToLower(path), ".env") || isEnvFile(path) {
		// Load .env file
		envMap, err := godotenv.Read(path)
		if err != nil {
			return nil, fmt.Errorf("load env file: %w", err)
		}

		// Map .env variables to config struct
		c.Database.URL = envMap["DATABASE_URL"]

		// Support discrete DB_* keys for backward compatibility
		if c.Database.URL == "" {
			host := envMap["DB_HOST"]
			port := envMap["DB_PORT"]
			name := envMap["DB_NAME"]
			user := envMap["DB_USER"]
			pass := envMap["DB_PASSWORD"]
			sslmode := envMap["DB_SSLMODE"]
			if sslmode == "" {
				sslmode = "disable"
			}
			if host != "" && port != "" && name != "" && user != "" {
				u := &url.URL{
					Scheme: "postgres",
					User:   url.UserPassword(user, pass),
					Host:   fmt.Sprintf("%s:%s", host, port),
					Path:   "/" + name,
					RawQuery: url.Values{"sslmode": []string{sslmode}}.Encode(),
				}
				c.Database.URL = u.String()
			}
		}

		// Coinbase configuration
		c.Coinbase.APIKey = envMap["COINBASE_API_KEY"]
		c.Coinbase.APISecret = envMap["COINBASE_API_SECRET"]
		c.Coinbase.Passphrase = envMap["COINBASE_PASSPHRASE"]
		c.Coinbase.APIKeyName = envMap["COINBASE_API_KEY_NAME"]
		if c.Coinbase.APIKeyName == "" {
			c.Coinbase.APIKeyName = envMap["COINBASE_CLOUD_API_KEY_NAME"]
		}
		c.Coinbase.APIPrivateKey = envMap["COINBASE_API_PRIVATE_KEY"]
		if c.Coinbase.APIPrivateKey == "" {
			c.Coinbase.APIPrivateKey = envMap["COINBASE_CLOUD_API_SECRET"]
		}

		// Rate limiting and retries
		if v := envMap["COINBASE_RPM"]; v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				c.Coinbase.RPM = parsed
			}
		}
		if v := envMap["COINBASE_MAX_RETRIES"]; v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				c.Coinbase.MaxRetries = parsed
			}
		}
		if v := envMap["COINBASE_BACKOFF_MS"]; v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				c.Coinbase.BackoffMS = parsed
			}
		}
	} else {
		// Load .ini file (existing logic)
		cfgfile, err := ini.Load(path)
		if err != nil {
			return nil, fmt.Errorf("load ini: %w", err)
		}

		// Backward compatible sections
		c.Database.URL = cfgfile.Section("database").Key("url").String()
		if c.Database.URL == "" {
			// Support user's [default] section with discrete DB_* keys
			def := cfgfile.Section("default")
			host := def.Key("DB_HOST").String()
			port := def.Key("DB_PORT").String()
			name := def.Key("DB_NAME").String()
			user := def.Key("DB_USER").String()
			pass := def.Key("DB_PASSWORD").String()
			sslmode := def.Key("DB_SSLMODE").String()
			if sslmode == "" {
				sslmode = "disable"
			}
			if host != "" && port != "" && name != "" && user != "" {
				// Compose a Postgres URL safely
				u := &url.URL{
					Scheme: "postgres",
					User:   url.UserPassword(user, pass),
					Host:   fmt.Sprintf("%s:%s", host, port),
					Path:   "/" + name,
					RawQuery: url.Values{"sslmode": []string{sslmode}}.Encode(),
				}
				c.Database.URL = u.String()
			}
		}
		coinbaseSec := cfgfile.Section("coinbase")
		c.Coinbase.APIKey = coinbaseSec.Key("api_key").String()
		c.Coinbase.APISecret = coinbaseSec.Key("api_secret").String()
		c.Coinbase.Passphrase = coinbaseSec.Key("passphrase").String()
		if c.Coinbase.APIKey == "" {
			def := cfgfile.Section("default")
			c.Coinbase.APIKey = def.Key("COINBASE_API_KEY").String()
			c.Coinbase.APISecret = def.Key("COINBASE_API_SECRET").String()
			c.Coinbase.Passphrase = def.Key("COINBASE_PASSPHRASE").String()
		}

		// Load credentials from JSON file if provided.
		// This is the preferred method and will override any other settings.
		if credsPath != "" {
			credsData, err := os.ReadFile(credsPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read coinbase credentials file: %w", err)
			}
			var creds CoinbaseCreds
			if err := json.Unmarshal(credsData, &creds); err != nil {
				return nil, fmt.Errorf("failed to unmarshal coinbase credentials json: %w", err)
			}
			c.Coinbase.APIKeyName = creds.Name
			c.Coinbase.APIPrivateKey = creds.PrivateKey
		} else {
			// Fallback to old method if no creds file is provided
			def := cfgfile.Section("default")
			c.Coinbase.APIKeyName = def.Key("COINBASE_API_KEY_NAME").String()
			if c.Coinbase.APIKeyName == "" {
				c.Coinbase.APIKeyName = def.Key("COINBASE_CLOUD_API_KEY_NAME").String()
			}
			c.Coinbase.APIPrivateKey = def.Key("COINBASE_API_PRIVATE_KEY").String()
			if c.Coinbase.APIPrivateKey == "" {
				c.Coinbase.APIPrivateKey = def.Key("COINBASE_CLOUD_API_SECRET").String()
			}
		}
		// Rate limiting and retries (prefer [coinbase], fallback to [default])
		if v, err := coinbaseSec.Key("rpm").Int(); err == nil {
			c.Coinbase.RPM = v
		}
		if v, err := coinbaseSec.Key("max_retries").Int(); err == nil {
			c.Coinbase.MaxRetries = v
		}
		if v, err := coinbaseSec.Key("backoff_ms").Int(); err == nil {
			c.Coinbase.BackoffMS = v
		}
		def := cfgfile.Section("default")
		if c.Coinbase.RPM == 0 {
			if v, err := def.Key("COINBASE_RPM").Int(); err == nil {
				c.Coinbase.RPM = v
			}
		}
		if c.Coinbase.MaxRetries == 0 {
			if v, err := def.Key("COINBASE_MAX_RETRIES").Int(); err == nil {
				c.Coinbase.MaxRetries = v
			}
		}
		if c.Coinbase.BackoffMS == 0 {
			if v, err := def.Key("COINBASE_BACKOFF_MS").Int(); err == nil {
				c.Coinbase.BackoffMS = v
			}
		}
	}

	// Load credentials from JSON file if provided (for both .env and .ini)
	// This is the preferred method and will override any other settings.
	if credsPath != "" {
		credsData, err := os.ReadFile(credsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read coinbase credentials file: %w", err)
		}
		var creds CoinbaseCreds
		if err := json.Unmarshal(credsData, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal coinbase credentials json: %w", err)
		}
		c.Coinbase.APIKeyName = creds.Name
		c.Coinbase.APIPrivateKey = creds.PrivateKey
	}

	// Defaults if still zero
	if c.Coinbase.MaxRetries == 0 {
		c.Coinbase.MaxRetries = 3
	}
	if c.Coinbase.BackoffMS == 0 {
		c.Coinbase.BackoffMS = 500
	}
	return &c, nil
}

// isEnvFile checks if a file appears to be an .env file by examining its content
func isEnvFile(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	envLines := 0
	iniSections := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for .env format (KEY=VALUE)
		if strings.Contains(line, "=") && !strings.HasPrefix(line, "[") && !strings.HasSuffix(line, "]") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[0] != "" {
				envLines++
			}
		}

		// Check for INI format ([section])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && len(line) > 2 {
			iniSections++
		}
	}

	// If we have more env-style lines than INI sections, consider it an .env file
	return envLines > iniSections
}

func splitAndTrim(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, trimSpaces(cur))
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, trimSpaces(cur))
	}
	return out
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
