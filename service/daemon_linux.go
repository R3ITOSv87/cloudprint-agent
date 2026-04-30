//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
)

const systemdUnit = `[Unit]
Description=CloudPrint Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=cloudprint
ExecStart=/usr/local/bin/cloudprint-agent run
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

// InstallService installs cloudprint-agent as a systemd service.
func InstallService() error {
	unitPath := "/etc/systemd/system/cloudprint-agent.service"

	if err := os.WriteFile(unitPath, []byte(systemdUnit), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	// Create system user if needed
	exec.Command("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", "cloudprint").Run()

	// Ensure binary is in PATH
	if _, err := exec.LookPath("cloudprint-agent"); err != nil {
		self, _ := os.Executable()
		if self != "" && self != "/usr/local/bin/cloudprint-agent" {
			data, _ := os.ReadFile(self)
			if err := os.WriteFile("/usr/local/bin/cloudprint-agent", data, 0755); err != nil {
				return fmt.Errorf("install binary: %w", err)
			}
		}
	}

	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload: %w — %s", err, out)
	}
	if out, err := exec.Command("systemctl", "enable", "cloudprint-agent").CombinedOutput(); err != nil {
		return fmt.Errorf("enable service: %w — %s", err, out)
	}

	fmt.Println("Service installed. Start with: sudo systemctl start cloudprint-agent")
	fmt.Println("View logs with:  journalctl -u cloudprint-agent -f")
	return nil
}

// UninstallService removes the systemd service.
func UninstallService() error {
	exec.Command("systemctl", "stop", "cloudprint-agent").Run()
	exec.Command("systemctl", "disable", "cloudprint-agent").Run()
	os.Remove("/etc/systemd/system/cloudprint-agent.service")
	exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("Service uninstalled.")
	return nil
}

// StartService starts the systemd service.
func StartService() error {
	out, err := exec.Command("systemctl", "start", "cloudprint-agent").CombinedOutput()
	if err != nil {
		return fmt.Errorf("start service: %w — %s", err, out)
	}
	fmt.Println("Service started.")
	return nil
}

// StopService stops the systemd service.
func StopService() error {
	out, err := exec.Command("systemctl", "stop", "cloudprint-agent").CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop service: %w — %s", err, out)
	}
	fmt.Println("Service stopped.")
	return nil
}
