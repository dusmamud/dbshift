package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/dusmamud/dbshift/internal/core"
)

// Config represents runtime parameters.
type Config struct {
	Engine       core.EngineType
	SourceURI    string
	TargetURI    string
	Mode         core.MigrationMode
	BatchSize    int
	Concurrency  int
	DropExisting bool
	Tables       []string
	Interactive  bool
}

// LoadConfig loads environment variables and sets defaults.
func LoadConfig() *Config {
	_ = godotenv.Load() // optional .env file

	cfg := &Config{
		Engine:      core.EnginePostgres,
		Mode:        core.ModeFull,
		BatchSize:   2500,
		Concurrency: 4,
	}

	if src := os.Getenv("SOURCE_URI"); src != "" {
		cfg.SourceURI = src
	}
	if tgt := os.Getenv("TARGET_URI"); tgt != "" {
		cfg.TargetURI = tgt
	}
	if eng := os.Getenv("DB_ENGINE"); eng != "" {
		if strings.ToLower(eng) == "mongo" || strings.ToLower(eng) == "mongodb" {
			cfg.Engine = core.EngineMongo
		}
	}
	if bs := os.Getenv("BATCH_SIZE"); bs != "" {
		if val, err := strconv.Atoi(bs); err == nil && val > 0 {
			cfg.BatchSize = val
		}
	}
	if conc := os.Getenv("CONCURRENCY"); conc != "" {
		if val, err := strconv.Atoi(conc); err == nil && val > 0 {
			cfg.Concurrency = val
		}
	}

	// Auto-detect engine if SOURCE_URI or TARGET_URI is set
	if cfg.SourceURI != "" {
		if eng := DetectEngineFromURI(cfg.SourceURI); eng != "" {
			cfg.Engine = eng
		}
	} else if cfg.TargetURI != "" {
		if eng := DetectEngineFromURI(cfg.TargetURI); eng != "" {
			cfg.Engine = eng
		}
	}

	return cfg
}

// DetectEngineFromURI determines engine type from URI scheme.
func DetectEngineFromURI(uri string) core.EngineType {
	clean := strings.ToLower(strings.TrimSpace(uri))
	if strings.HasPrefix(clean, "postgres://") || strings.HasPrefix(clean, "postgresql://") {
		return core.EnginePostgres
	}
	if strings.HasPrefix(clean, "mongodb://") || strings.HasPrefix(clean, "mongodb+srv://") {
		return core.EngineMongo
	}
	return ""
}
