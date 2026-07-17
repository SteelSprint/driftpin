package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	DatabasePath    string `json:"databasePath"`
	Currency        string `json:"currency"`
	LowStockThreshold int  `json:"lowStockThreshold"`
}

// D! id=cfg_def range-start
func DefaultConfig() Config {
	return Config{
		DatabasePath:      "inventory.json",
		Currency:          "USD",
		LowStockThreshold: 5,
	}
}
// D! id=cfg_def range-end

// D! id=cfg_load range-start
func LoadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = DefaultConfig().DatabasePath
	}
	if cfg.Currency == "" {
		cfg.Currency = DefaultConfig().Currency
	}
	if cfg.LowStockThreshold == 0 {
		cfg.LowStockThreshold = DefaultConfig().LowStockThreshold
	}
	return cfg
}
// D! id=cfg_load range-end
