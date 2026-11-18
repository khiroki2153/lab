package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ProxyConfig struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type AdminConfig struct {
	Port int `yaml:"port"`
}

type Config struct {
	Proxies []ProxyConfig `yaml:"proxies"`
	Admin   AdminConfig   `yaml:"admin"`
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Proxies) == 0 {
		return nil, fmt.Errorf("no proxies defined in config")
	}
	return &cfg, nil
}
