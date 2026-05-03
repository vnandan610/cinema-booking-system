package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sikozonpc/cinema/internal/platform/cache"
)

type Settings struct {
	HTTP        HTTPSettings
	Redis       cache.RedisSettings
	Reservation ReservationSettings
	StaticDir   string
	LogLevel    slog.Level
}

type HTTPSettings struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type ReservationSettings struct {
	HoldTTL time.Duration
}

func Load() Settings {
	return Settings{
		HTTP: HTTPSettings{
			Addr:              ":" + getEnv("PORT", "8080"),
			ReadHeaderTimeout: getDuration("HTTP_READ_HEADER_TIMEOUT", 3*time.Second),
			ReadTimeout:       getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      getDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		Redis: cache.RedisSettings{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Username:     os.Getenv("REDIS_USERNAME"),
			Password:     os.Getenv("REDIS_PASSWORD"),
			DB:           getInt("REDIS_DB", 0),
			PoolSize:     getInt("REDIS_POOL_SIZE", 20),
			MinIdleConns: getInt("REDIS_MIN_IDLE_CONNS", 2),
		},
		Reservation: ReservationSettings{
			HoldTTL: getDuration("HOLD_TTL", 2*time.Minute),
		},
		StaticDir: getEnv("STATIC_DIR", "static"),
		LogLevel:  parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
