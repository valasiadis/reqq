package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TODO default config

type MailConfig struct {
	Server   string
	Port     int
	User     string
	Password string
	Prefix   string
}

type TurnstileConfig struct {
	EnforceValidation bool   `yaml:"enforce_validation"`
	Secret            string `yaml:"secret"`
}

type RedirectConfig struct {
	Success string
	Error   struct {
		Generic   string
		Turnstile string
		Mail      string
	}
}

type DepartmentConfig struct {
	Display string `yaml:"display_name"`
	Email   string
}

type Config struct {
	ListenAddress string `yaml:"listen_address"`
	Mail          MailConfig
	Turnstile     TurnstileConfig
	Redirect      RedirectConfig
	Departments   map[string]DepartmentConfig `yaml:"departments"`
}

func getConfig(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config *Config
	yaml.Unmarshal(content, &config)
	// little easter egg
	config.Departments["e-11"] = DepartmentConfig{Display: "E-11 Blastergewehr", Email: "emanuel@valasiadis.space"}
	return config, nil
}
