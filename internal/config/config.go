package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// ServiceConfig holds a service URL and its route alias.
type ServiceConfig struct {
	URL   string `yaml:"url"`
	Alias string `yaml:"alias"`
}

type ProxyConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type YAMLConfig struct {
	Proxy    ProxyConfig     `yaml:"proxy"`
	Services []ServiceConfig `yaml:"services"`
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*YAMLConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg YAMLConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	log.Printf("[CONFIG] Loaded config from: %s", path)
	log.Println(cfg)
	return &cfg, nil
}
