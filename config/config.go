package config

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Config struct {
	Port                        int
	Verbosity                   int
	PostgresConnStr             string
	ScriptsDir                  string
	LatestHours                 int
	ActiveAddressHours          int
	MaxReqCount                 int
	ReqTimeoutSec               int
	DefaultCacheMaxItemCount    int
	DefaultCacheItemLifeTimeSec int
	PerformanceMonitor          PerformanceMonitorConfig
	FrozenBalanceAddrs          []string
	ReqsPerMinuteLimit          int
	DynamicConfigFile           string
	Swagger                     SwaggerConfig
	Indexer                     IndexerConfig
	LogFileSize                 int
	ChangeLogUrl                string
	DynamicEndpointsTable       string
	DynamicEndpointStatesTable  string
	Cors                        bool
}

type IndexerConfig struct {
	Url            string
	MaxConnections int
}

type PerformanceMonitorConfig struct {
	Enabled     bool
	Interval    string
	LogFileSize int
}

type SwaggerConfig struct {
	Enabled  bool
	Host     string
	BasePath string
}

func LoadConfig(configPath string) *Config {
	if _, err := os.Stat(configPath); err != nil {
		panic(errors.Errorf("Config file cannot be found, path: %v", configPath))
	}
	if jsonFile, err := os.Open(configPath); err != nil {
		panic(errors.Errorf("Config file cannot be opened, path: %v", configPath))
	} else {
		conf := newDefaultConfig()
		byteValue, _ := ioutil.ReadAll(jsonFile)
		err := json.Unmarshal(byteValue, conf)
		if err != nil {
			panic(errors.Errorf("Cannot parse JSON config, path: %v", configPath))
		}
		return conf
	}
}

func newDefaultConfig() *Config {
	return &Config{
		ScriptsDir:                  filepath.Join("resources", "scripts", "api"),
		Verbosity:                   3,
		LatestHours:                 24,
		ActiveAddressHours:          24,
		MaxReqCount:                 50,
		ReqTimeoutSec:               60,
		DefaultCacheMaxItemCount:    100,
		DefaultCacheItemLifeTimeSec: 60,
		ReqsPerMinuteLimit:          0,
		DynamicConfigFile:           filepath.Join("conf", "apiDynamic.json"),
		Swagger: SwaggerConfig{
			Enabled: false,
		},
		LogFileSize: 1024 * 100,
		Cors:        true,
	}
}
