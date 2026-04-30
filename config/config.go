package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type PrinterConfig struct {
	CloudPrinterID string                 `json:"cloud_printer_id"`
	LocalName      string                 `json:"local_name"`
	IPPURI         string                 `json:"ipp_uri"`
	Method         string                 `json:"method"` // "ipp" or "cups"
	Capabilities   map[string]interface{} `json:"capabilities,omitempty"`
}

type Config struct {
	APIURL            string          `json:"api_url"`
	AgentID           string          `json:"agent_id"`
	APIKey            string          `json:"api_key"`
	AgentName         string          `json:"agent_name"`
	PollInterval      int             `json:"poll_interval_seconds"`
	HeartbeatInterval int             `json:"heartbeat_interval_seconds"`
	LogLevel          string          `json:"log_level"`
	Printers          []PrinterConfig `json:"printers"`
}

func DefaultPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("ProgramData"), "CloudPrint", "agent.json")
	}
	if os.Getuid() == 0 {
		return "/etc/cloudprint/agent.json"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cloudprint", "agent.json")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 30
	}
	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func New(apiURL, agentID, apiKey, name string) *Config {
	return &Config{
		APIURL:            apiURL,
		AgentID:           agentID,
		APIKey:            apiKey,
		AgentName:         name,
		PollInterval:      5,
		HeartbeatInterval: 30,
		LogLevel:          "info",
		Printers:          []PrinterConfig{},
	}
}
