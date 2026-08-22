package config

import (
	"errors"
	"os"
	"strings"

	"github.com/gopherust-io/env"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func Load() (Config, error) {
	err := env.LoadDotEnv(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	// LoadDotEnv mutates process env; v0.6+ LoadConfig reads Snapshot without Reload.
	env.Reload()

	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}

	applyDontRandomizeCompat(&cfg)

	err = cfg.Validate()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// applyDontRandomizeCompat honors legacy NATS_DDNT_RANDOMIZE when the corrected
// NATS_DONT_RANDOMIZE key is unset (LoadConfig / go:generate prefer DONT).
func applyDontRandomizeCompat(cfg *Config) {
	if raw, ok := os.LookupEnv("NATS_DONT_RANDOMIZE"); ok && !commonstrings.IsEmpty(strings.TrimSpace(raw)) {
		return
	}
	raw, ok := os.LookupEnv("NATS_DDNT_RANDOMIZE")
	if !ok || commonstrings.IsEmpty(strings.TrimSpace(raw)) {
		return
	}
	v, err := env.ParseBool(raw)
	if err != nil {
		return
	}
	cfg.NATS.DontRandomize = v
}
