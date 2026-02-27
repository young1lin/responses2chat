package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server       ServerConfig             `mapstructure:"server"`
	DefaultTarget TargetConfig             `mapstructure:"default_target"`
	Providers    map[string]TargetConfig  `mapstructure:"providers"`
	Logging      LoggingConfig            `mapstructure:"logging"`
	ModelMapping map[string]string        `mapstructure:"model_mapping"`
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type TargetConfig struct {
	BaseURL       string `mapstructure:"base_url"`
	PathSuffix    string `mapstructure:"path_suffix"`
	DefaultAPIKey string `mapstructure:"default_api_key"`
	Timeout       int    `mapstructure:"timeout"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load(cfgFile string) *Config {
	// Load .env file if exists (ignore error if not found)
	godotenv.Load()
	godotenv.Load(".env.local")

	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Bind environment variables
	v.SetEnvPrefix("R2C")
	v.AutomaticEnv()

	// Read config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
	}

	if err := v.ReadInConfig(); err != nil {
		// Config file not found is ok, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic("Error reading config file: " + err.Error())
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic("Error unmarshaling config: " + err.Error())
	}

	return &cfg
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 300)

	// Default target defaults
	v.SetDefault("default_target.path_suffix", "/v1/chat/completions")
	v.SetDefault("default_target.timeout", 300)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")
}
