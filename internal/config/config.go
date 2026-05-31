package config

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Port             string `toml:"port"`
	DBPath           string `toml:"db_path"`
	DevMode          bool   `toml:"dev_mode"`            // SROUTER_DEV_MODE=true
	UpdateIntervalMs int    `toml:"update_interval_ms"` // SROUTER_UPDATE_INTERVAL_MS=1000
}

func Load(path string) (Config, error) {
	cfg := Config{
		Port:             ":8080",
		DBPath:           "/var/lib/srouter/data.db",
		UpdateIntervalMs: 2000,
	}
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	applyEnv(&cfg)
	return cfg, nil
}

// applyEnv overrides config fields from environment variables.
// The env var name is derived from the toml tag: "db_path" -> "SROUTER_DB_PATH".
func applyEnv(cfg *Config) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("toml")
		if tag == "" || tag == "-" {
			continue
		}
		envKey := "SROUTER_" + strings.ToUpper(strings.ReplaceAll(tag, "-", "_"))
		val := os.Getenv(envKey)
		if val == "" {
			continue
		}
		switch v.Field(i).Kind() {
		case reflect.String:
			v.Field(i).SetString(val)
		case reflect.Bool:
			v.Field(i).SetBool(val == "true" || val == "1")
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				v.Field(i).SetInt(n)
			}
		}
	}
}
