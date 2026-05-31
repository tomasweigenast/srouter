package config

import (
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Port   string `toml:"port"`
	DBPath string `toml:"db_path"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		Port:   ":8080",
		DBPath: "/var/lib/srouter/data.db",
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
		if val := os.Getenv(envKey); val != "" && v.Field(i).Kind() == reflect.String {
			v.Field(i).SetString(val)
		}
	}
}
