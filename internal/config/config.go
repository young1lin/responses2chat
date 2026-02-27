package config

import (
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig            `mapstructure:"server"`
	DefaultTarget TargetConfig            `mapstructure:"default_target"`
	Providers     map[string]TargetConfig `mapstructure:"providers"`
	Logging       LoggingConfig           `mapstructure:"logging"`
	ModelMapping  map[string]string       `mapstructure:"model_mapping"`
	Storage       StorageConfig           `mapstructure:"storage"`
	WebSearch     WebSearchConfig         `mapstructure:"web_search"`
}

// WebSearchConfig represents web search configuration
type WebSearchConfig struct {
	Enabled   bool                      `mapstructure:"enabled"`
	Default   string                    `mapstructure:"default"` // Default provider name
	Providers map[string]ProviderConfig `mapstructure:"providers"`
}

// ProviderConfig represents a generic search provider configuration
type ProviderConfig struct {
	Type       string `mapstructure:"type"` // "mcp", "firecrawl", "rest"
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	ToolName   string `mapstructure:"tool_name"`   // MCP: tool name to call
	QueryParam string `mapstructure:"query_param"` // MCP: query parameter name
	Timeout    int    `mapstructure:"timeout"`
	MaxResults int    `mapstructure:"max_results"` // For firecrawl etc.
}

type StorageConfig struct {
	Path string `mapstructure:"path"` // Database path, default ./data/conversations.db
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type TargetConfig struct {
	BaseURL               string `mapstructure:"base_url"`
	PathSuffix            string `mapstructure:"path_suffix"`
	DefaultAPIKey         string `mapstructure:"default_api_key"`
	Timeout               int    `mapstructure:"timeout"`
	SupportsDeveloperRole bool   `mapstructure:"supports_developer_role"` // Whether provider supports 'developer' role
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

	// Configure environment variable handling
	// Replace . with _ for nested config keys
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("R2C")
	v.AutomaticEnv()

	// Read config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")          // Same directory as executable (priority)
		v.AddConfigPath("./configs")  // configs/ subdirectory
		v.AddConfigPath("../configs") // For running from bin/ directory
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

	// Storage defaults
	v.SetDefault("storage.path", "./data/conversations.db")

	// Web Search defaults
	v.SetDefault("web_search.enabled", true)
	v.SetDefault("web_search.default", "zhipu")
	v.SetDefault("web_search.providers.firecrawl.type", "firecrawl")
	v.SetDefault("web_search.providers.firecrawl.base_url", "https://api.firecrawl.dev/v2")
	v.SetDefault("web_search.providers.firecrawl.timeout", 30)
	v.SetDefault("web_search.providers.firecrawl.max_results", 5)
	v.SetDefault("web_search.providers.zhipu.type", "mcp")
	v.SetDefault("web_search.providers.zhipu.base_url", "https://open.bigmodel.cn/api/mcp/web_search_prime/mcp")
	v.SetDefault("web_search.providers.zhipu.tool_name", "webSearchPrime")
	v.SetDefault("web_search.providers.zhipu.query_param", "search_query")
	v.SetDefault("web_search.providers.zhipu.timeout", 30)
}
