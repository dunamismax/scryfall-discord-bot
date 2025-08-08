package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DiscordToken    string
	CommandPrefix   string
	LogLevel        string
	BotName         string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
	MaxRetries      int
	DebugMode       bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		CommandPrefix:   "!",    // default prefix
		LogLevel:        "info", // default log level
		BotName:         getEnv("BOT_NAME", "mtg-card-bot"),
		ShutdownTimeout: 30 * time.Second, // default shutdown timeout
		RequestTimeout:  30 * time.Second, // default request timeout
		MaxRetries:      3,                // default max retries
		DebugMode:       false,            // default debug mode
	}

	// Discord token is required
	cfg.DiscordToken = os.Getenv("DISCORD_TOKEN")
	if cfg.DiscordToken == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN environment variable is required")
	}

	// Optional configurations
	if prefix := os.Getenv("COMMAND_PREFIX"); prefix != "" {
		cfg.CommandPrefix = prefix
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = strings.ToLower(logLevel)
	}

	// Parse timeout configurations
	if timeout := os.Getenv("SHUTDOWN_TIMEOUT"); timeout != "" {
		if parsed, err := time.ParseDuration(timeout); err == nil {
			cfg.ShutdownTimeout = parsed
		} else {
			log.Printf("Warning: invalid SHUTDOWN_TIMEOUT format '%s', using default", timeout)
		}
	}

	if timeout := os.Getenv("REQUEST_TIMEOUT"); timeout != "" {
		if parsed, err := time.ParseDuration(timeout); err == nil {
			cfg.RequestTimeout = parsed
		} else {
			log.Printf("Warning: invalid REQUEST_TIMEOUT format '%s', using default", timeout)
		}
	}

	// Parse retry configuration
	cfg.MaxRetries = GetInt("MAX_RETRIES", cfg.MaxRetries)

	// Parse debug mode
	cfg.DebugMode = GetBool("DEBUG", cfg.DebugMode)

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DiscordToken == "" {
		return fmt.Errorf("discord token is required")
	}

	if c.CommandPrefix == "" {
		return fmt.Errorf("command prefix cannot be empty")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.LogLevel) {
		return fmt.Errorf("invalid log level: %s (valid: %s)", c.LogLevel, strings.Join(validLogLevels, ", "))
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}

	if c.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	return nil
}

// GetBool returns a boolean environment variable with a default value
func GetBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolVal
}

// GetInt returns an integer environment variable with a default value
func GetInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intVal
}

// getEnv returns an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
