// Package store is ghu's persistence adapter: it reads and writes
// ~/.ghu/config.yaml and nothing else.
//
// It is separate from the feature that uses it so that the yaml driver stays
// out of the business rules -- profiles decides what a valid set of profiles
// is; store only knows how one is spelled on disk.
package core

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/semirm-dev/ghu/internal/core/sys"
)

// Template is the starter config written when none exists. It is embedded
// rather than read from disk: reading a fixture relative to the working
// directory failed whenever ghu ran outside its own source tree.
//
//go:embed template.yaml
var Template []byte

// ErrVersion marks a config this ghu cannot read -- written by a newer one, or
// hand-edited to a version that does not exist. It is never tolerated:
// reconciling from a config ghu cannot parse would rewrite ~/.gitconfig from an
// empty profile list.
var ErrVersion = errors.New("config version is not supported")

// Load reads the config file. A missing or empty file yields an empty config,
// so first run needs no special casing.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{Version: ConfigVersion}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return Config{Version: ConfigVersion}, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Version != ConfigVersion {
		return Config{}, fmt.Errorf(
			"%s has version %d, this ghu understands version %d: %w",
			path, cfg.Version, ConfigVersion, ErrVersion)
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	cfg.Version = ConfigVersion

	if err := cfg.Validate(); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return sys.WriteAtomic(path, data, 0o600)
}

// BackupConfig copies the config file alongside itself, timestamped, before
// something overwrites it. Returns the empty string when there was no config
// to keep.
func BackupConfig(path string, now time.Time) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	dst := path + "." + now.UTC().Format(snapshotStamp)
	if err := sys.WriteAtomic(dst, data, 0o600); err != nil {
		return "", err
	}
	return dst, nil
}

func EnsureConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return sys.WriteAtomic(path, Template, 0o600)
}
