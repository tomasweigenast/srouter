package env

import "os"

var configPath string = "/etc/srouter/config.toml"

func init() {
	if env, exists := os.LookupEnv("SROUTER_CONFIG_PATH"); exists {
		configPath = env
	}
}

func ConfigPath() string {
	return configPath
}
