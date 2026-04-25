package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Framework       string         `json:"framework"`
	Include         []string       `json:"include"`
	ExcludeDirs     []string       `json:"excludeDirs"`
	ControllerPaths []string       `json:"controllerPaths"`
	DtoPaths        []string       `json:"dtoPaths"`
	ServicePaths    []string       `json:"servicePaths"`
	DtoMarkers      []string       `json:"dtoMarkers"`
	ModulePattern   string         `json:"modulePattern"`
	OpenAPI         OpenAPIConfig  `json:"openapi"`
	Security        SecurityConfig `json:"security"`
	Server          ServerConfig   `json:"server"`
}

type OpenAPIConfig struct {
	File string `json:"file"`
	URL  string `json:"url"`
}

type SecurityConfig struct {
	AllowSourceTools bool `json:"allowSourceTools"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func defaultConfig() Config {
	return Config{
		Framework: "spring",
		Include:   []string{"**/*.java"},
		ExcludeDirs: []string{
			".git", "node_modules", ".idea", "target", ".mvn", "dist", "build",
			".vscode", ".mcp-backend-context", ".gradle", "out",
		},
		ControllerPaths: []string{"controller", "controllers", "api", "resource", "resources", "web"},
		DtoPaths:        []string{"vo", "dto", "dtos", "model", "models", "request", "response", "payload", "entity"},
		ServicePaths:    []string{"service", "services", "application", "usecase", "usecases"},
		DtoMarkers:      []string{"VO", "Vo", "DTO", "Dto", "Request", "Response", "Payload", "Command", "Query", "Model"},
		ModulePattern:   `(?:^|/)(?:[\w-]+-)?(?:module|service|app|api)-([\w-]+)(?:/|$)`,
		Security: SecurityConfig{
			AllowSourceTools: false,
		},
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 3100,
		},
	}
}

func loadConfig(root, configPath string) (Config, error) {
	cfg := defaultConfig()
	if configPath == "" {
		for _, name := range []string{"backend-context.config.json", ".backend-context.json"} {
			candidate := filepath.Join(root, name)
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}
		}
	}
	if configPath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	mergeConfigDefaults(&cfg)
	return cfg, nil
}

func mergeConfigDefaults(cfg *Config) {
	def := defaultConfig()
	if cfg.Framework == "" {
		cfg.Framework = def.Framework
	}
	if len(cfg.Include) == 0 {
		cfg.Include = def.Include
	}
	if len(cfg.ExcludeDirs) == 0 {
		cfg.ExcludeDirs = def.ExcludeDirs
	}
	if len(cfg.ControllerPaths) == 0 {
		cfg.ControllerPaths = def.ControllerPaths
	}
	if len(cfg.DtoPaths) == 0 {
		cfg.DtoPaths = def.DtoPaths
	}
	if len(cfg.ServicePaths) == 0 {
		cfg.ServicePaths = def.ServicePaths
	}
	if len(cfg.DtoMarkers) == 0 {
		cfg.DtoMarkers = def.DtoMarkers
	}
	if cfg.ModulePattern == "" {
		cfg.ModulePattern = def.ModulePattern
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = def.Server.Host
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = def.Server.Port
	}
}
