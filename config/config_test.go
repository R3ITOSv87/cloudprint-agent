package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.json")

	cfg := New("https://api.base44.com/api/apps/test/functions", "agent-123", "sk-agent-abc", "Test Agent")
	cfg.Printers = []PrinterConfig{
		{
			CloudPrinterID: "printer-1",
			LocalName:      "HP LaserJet",
			IPPURI:         "ipp://192.168.1.50:631/ipp/print",
			Method:         "ipp",
		},
	}

	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// File should exist with restricted permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file mode 0600, got %o", info.Mode().Perm())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.AgentID != "agent-123" {
		t.Errorf("AgentID = %q, want %q", loaded.AgentID, "agent-123")
	}
	if loaded.APIKey != "sk-agent-abc" {
		t.Errorf("APIKey = %q, want %q", loaded.APIKey, "sk-agent-abc")
	}
	if loaded.AgentName != "Test Agent" {
		t.Errorf("AgentName = %q, want %q", loaded.AgentName, "Test Agent")
	}
	if loaded.PollInterval != 5 {
		t.Errorf("PollInterval = %d, want 5", loaded.PollInterval)
	}
	if loaded.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval = %d, want 30", loaded.HeartbeatInterval)
	}
	if len(loaded.Printers) != 1 {
		t.Fatalf("Printers len = %d, want 1", len(loaded.Printers))
	}
	if loaded.Printers[0].IPPURI != "ipp://192.168.1.50:631/ipp/print" {
		t.Errorf("Printer IPPURI = %q", loaded.Printers[0].IPPURI)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("/nonexistent/path/agent.json")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestDefaultPollIntervals(t *testing.T) {
	cfg := New("https://example.com", "id", "key", "name")
	if cfg.PollInterval != 5 {
		t.Errorf("default PollInterval = %d, want 5", cfg.PollInterval)
	}
	if cfg.HeartbeatInterval != 30 {
		t.Errorf("default HeartbeatInterval = %d, want 30", cfg.HeartbeatInterval)
	}
}
