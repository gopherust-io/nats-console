package config

import (
	"errors"
	"os"

	"github.com/gopherust-io/env"
)

func Load() (Config, error) {
	err := env.LoadDotEnv(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	cfg, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}

	err = cfg.Validate()
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
