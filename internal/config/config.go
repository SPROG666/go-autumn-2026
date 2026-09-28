package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr                string
	HTTPReadTimeout         time.Duration
	HTTPReadHeaderTimeout   time.Duration
	HTTPWriteTimeout        time.Duration
	HTTPIdleTimeout         time.Duration
	LogLevel                slog.Level
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	IdempotencyTTL          time.Duration
}

func Load() (Config, error) {
	var cfg Config
	var err error

	cfg.HTTPAddr, err = required("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR: expected host:port or :port: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR: parse port: %w", err)
	}
	if portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR: port must be between 1 and 65535")
	}

	cfg.DatabaseURL, err = required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	level, err := required("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL: invalid log level: %w", err)
	}

	for _, setting := range []struct {
		name   string
		target *time.Duration
	}{
		{"HTTP_READ_TIMEOUT", &cfg.HTTPReadTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", &cfg.HTTPReadHeaderTimeout},
		{"HTTP_WRITE_TIMEOUT", &cfg.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", &cfg.HTTPIdleTimeout},
		{"SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout},
		{"DATABASE_MAX_CONN_LIFETIME", &cfg.DatabaseMaxConnLifetime},
		{"DATABASE_CONNECT_TIMEOUT", &cfg.DatabaseConnectTimeout},
		{"DATABASE_QUERY_TIMEOUT", &cfg.DatabaseQueryTimeout},
		{"IDEMPOTENCY_TTL", &cfg.IdempotencyTTL},
	} {
		value, err := required(setting.name)
		if err != nil {
			return Config{}, err
		}
		duration, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("%s: parse duration: %w", setting.name, err)
		}
		if duration <= 0 {
			return Config{}, fmt.Errorf("%s: expected a positive duration, for example 5s", setting.name)
		}
		*setting.target = duration
	}

	cfg.DatabaseMaxConns, err = connections("DATABASE_MAX_CONNS", 1)
	if err != nil {
		return Config{}, err
	}
	cfg.DatabaseMinConns, err = connections("DATABASE_MIN_CONNS", 0)
	if err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMinConns > cfg.DatabaseMaxConns {
		return Config{}, fmt.Errorf("DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}

	return cfg, nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s: required environment variable is empty or missing", name)
	}
	return value, nil
}

func connections(name string, minimum int64) (int32, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	number, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: parse connection count: %w", name, err)
	}
	if number < minimum {
		return 0, fmt.Errorf("%s: expected an int32 value >= %d", name, minimum)
	}
	return int32(number), nil
}
