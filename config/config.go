package config

import (
	"github.com/spf13/viper"
)

// Config stores all configuration of the application.
// The values are read by viper from a config file or environment variable.
type Config struct {
	GeminiAPIKey      string `mapstructure:"GEMINI_API_KEY"`
	GRPCServerAddress string `mapstructure:"GRPC_SERVER_ADDRESS"`
}

// LoadConfig reads configuration from file or environment variables.

func loadAncScan(config *Config) (err error) {
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	err = viper.Unmarshal(config)
	if err != nil {
		return err
	}
	return nil
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (config Config, err error) {

	viper.AddConfigPath(path)
	viper.SetConfigName("dev")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	err = loadAncScan(&config)
	if err != nil {
		return
	}
	return
}
