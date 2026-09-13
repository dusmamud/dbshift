package config_test

import (
	"testing"

	"github.com/dusmamud/dbshift/internal/config"
	"github.com/dusmamud/dbshift/internal/core"
)

func TestDetectEngineFromURI(t *testing.T) {
	cases := []struct {
		uri      string
		expected core.EngineType
	}{
		{
			uri:      "postgresql://username:password@example.com:26257/defaultdb?sslmode=verify-full",
			expected: core.EnginePostgres,
		},
		{
			uri:      "postgres://username:password@example.com:5432/postgres",
			expected: core.EnginePostgres,
		},
		{
			uri:      "mongodb+srv://username:password@example.com/production?retryWrites=true",
			expected: core.EngineMongo,
		},
		{
			uri:      "mongodb://localhost:27017/analytics",
			expected: core.EngineMongo,
		},
		{
			uri:      "mysql://root:pass@localhost:3306/db",
			expected: "",
		},
	}

	for _, c := range cases {
		got := config.DetectEngineFromURI(c.uri)
		if got != c.expected {
			t.Errorf("DetectEngineFromURI(%q) = %q; want %q", c.uri, got, c.expected)
		}
	}
}
