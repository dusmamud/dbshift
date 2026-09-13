package mongo

import (
	"testing"
)

func TestExtractMongoDbName(t *testing.T) {
	cases := []struct {
		uri      string
		expected string
	}{
		{
			uri:      "mongodb+srv://username:password@example.com/production?retryWrites=true&w=majority",
			expected: "production",
		},
		{
			uri:      "mongodb://localhost:27017/analytics",
			expected: "analytics",
		},
		{
			uri:      "mongodb://localhost:27017",
			expected: "test",
		},
	}

	for _, c := range cases {
		got := extractMongoDbName(c.uri)
		if got != c.expected {
			t.Errorf("extractMongoDbName(%q) = %q; want %q", c.uri, got, c.expected)
		}
	}
}
