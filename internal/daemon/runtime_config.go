package daemon

import "github.com/iamtew/t3b/internal/config"

func loadRuntimeConfig(configPath string) (*config.Config, error) {
	if configPath == "" {
		configPath = config.DefaultPath
	}
	return config.Load(configPath)
}
