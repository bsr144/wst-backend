package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate_ProductionGates(t *testing.T) {
	t.Parallel()

	base := Config{
		AppEnv:             "production",
		CORSOrigins:        "https://app.example.com",
		BodyLimitBytes:     6 << 20,
		UploadLimitBytes:   5 << 20,
		UploadAllowedTypes: []string{"image/jpeg", "image/png", "application/pdf"},
		Postgres:           Postgres{SSLMode: "require", Password: "secret"},
		MinIO:              MinIO{AccessKey: "key", SecretKey: "secret"},
	}

	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{"valid production", func(c *Config) {}, false},
		{"upload limit not smaller than body limit rejected", func(c *Config) { c.UploadLimitBytes = int64(c.BodyLimitBytes) }, true},
		{"empty allowed upload types rejected", func(c *Config) { c.UploadAllowedTypes = nil }, true},
		{"sslmode disable rejected", func(c *Config) { c.Postgres.SSLMode = "disable" }, true},
		{"cors wildcard rejected", func(c *Config) { c.CORSOrigins = "*" }, true},
		{"empty db password rejected", func(c *Config) { c.Postgres.Password = "" }, true},
		{"empty minio creds rejected", func(c *Config) { c.MinIO.AccessKey = "" }, true},
		{"non-production skips gates", func(c *Config) {
			c.AppEnv = "local"
			c.Postgres.SSLMode = "disable"
			c.CORSOrigins = "*"
		}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := base
			tc.mutate(&c)
			err := c.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
