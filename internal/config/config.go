package config

import (
	"context"
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
	"net/url"

	ini "gopkg.in/ini.v1"
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
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".config", "crypto-thing", "config.ini")
	}
	cfgfile, err := ini.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load ini: %w", err)
	}
	var c Config
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
	// Defaults if still zero
	if c.Coinbase.MaxRetries == 0 {
		c.Coinbase.MaxRetries = 3
	}
	if c.Coinbase.BackoffMS == 0 {
		c.Coinbase.BackoffMS = 500
	}
		return &c, nil
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
