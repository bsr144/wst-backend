package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Config struct {
	AppEnv             string
	AppPort            string
	LogLevel           string
	CORSOrigins        string
	RequestTimeout     time.Duration
	ShutdownTimeout    time.Duration
	BodyLimitBytes     int
	UploadLimitBytes   int64
	UploadAllowedTypes []string

	Postgres  Postgres
	MinIO     MinIO
	Pricing   Pricing
	Worker    Worker
	RateLimit RateLimit
}

type Postgres struct {
	Host              string
	Port              string
	User              string
	Password          string
	DB                string
	SSLMode           string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	StatementTimeout  time.Duration
}

func (p Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode)
}

type MinIO struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type Pricing struct {
	Standard   decimal.Decimal
	Electronic decimal.Decimal
}

type Worker struct {
	SweepInterval time.Duration
	OrganicTTL    time.Duration
}

type RateLimit struct {
	RPM   int
	Burst int
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:             env("APP_ENV", "local"),
		AppPort:            env("APP_PORT", "8080"),
		LogLevel:           env("LOG_LEVEL", "info"),
		CORSOrigins:        env("CORS_ORIGINS", "*"),
		RequestTimeout:     envDuration("REQUEST_TIMEOUT", 5*time.Second),
		ShutdownTimeout:    envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		BodyLimitBytes:     int(envInt64("BODY_LIMIT_BYTES", 6<<20)),
		UploadLimitBytes:   envInt64("UPLOAD_LIMIT_BYTES", 5<<20),
		UploadAllowedTypes: envStrings("UPLOAD_ALLOWED_TYPES", "image/jpeg,image/png,application/pdf"),
		Postgres: Postgres{
			Host:              env("POSTGRES_HOST", "localhost"),
			Port:              env("POSTGRES_PORT", "5432"),
			User:              env("POSTGRES_USER", "wst"),
			Password:          env("POSTGRES_PASSWORD", "wst"),
			DB:                env("POSTGRES_DB", "wst"),
			SSLMode:           env("POSTGRES_SSLMODE", "disable"),
			MaxConns:          int32(envInt64("POSTGRES_MAX_CONNS", 10)),
			MinConns:          int32(envInt64("POSTGRES_MIN_CONNS", 2)),
			MaxConnLifetime:   envDuration("POSTGRES_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   envDuration("POSTGRES_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: envDuration("POSTGRES_HEALTHCHECK_PERIOD", time.Minute),
			StatementTimeout:  envDuration("POSTGRES_STATEMENT_TIMEOUT", 5*time.Second),
		},
		MinIO: MinIO{
			Endpoint:  env("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: env("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: env("MINIO_SECRET_KEY", "minioadmin"),
			Bucket:    env("MINIO_BUCKET", "proofs"),
			UseSSL:    envBool("MINIO_USE_SSL", false),
		},
		Worker: Worker{
			SweepInterval: envDuration("SWEEP_INTERVAL", time.Minute),
			OrganicTTL:    envDuration("ORGANIC_TTL", 72*time.Hour),
		},
		RateLimit: RateLimit{
			RPM:   int(envInt64("PICKUP_RATE_RPM", 30)),
			Burst: int(envInt64("PICKUP_RATE_BURST", 10)),
		},
	}

	standard, err := envDecimal("PRICE_STANDARD", "10000")
	if err != nil {
		return Config{}, err
	}
	electronic, err := envDecimal("PRICE_ELECTRONIC", "50000")
	if err != nil {
		return Config{}, err
	}
	cfg.Pricing = Pricing{Standard: standard, Electronic: electronic}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.UploadLimitBytes >= int64(c.BodyLimitBytes) {
		return errors.New("config: UPLOAD_LIMIT_BYTES must be smaller than BODY_LIMIT_BYTES")
	}
	if len(c.UploadAllowedTypes) == 0 {
		return errors.New("config: UPLOAD_ALLOWED_TYPES must list at least one content type")
	}
	if c.AppEnv != "production" {
		return nil
	}
	if c.Postgres.SSLMode == "disable" {
		return errors.New("config: POSTGRES_SSLMODE=disable is not allowed in production")
	}
	if c.CORSOrigins == "*" {
		return errors.New("config: CORS_ORIGINS=* is not allowed in production")
	}
	if c.Postgres.Password == "" {
		return errors.New("config: POSTGRES_PASSWORD must be set in production")
	}
	if c.MinIO.AccessKey == "" || c.MinIO.SecretKey == "" {
		return errors.New("config: MINIO credentials must be set in production")
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envStrings(key, def string) []string {
	raw := env(key, def)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func envInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envDecimal(key, def string) (decimal.Decimal, error) {
	return decimal.NewFromString(env(key, def))
}
