//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallService installs cloudprint-agent as a Windows service via sc.exe.
func InstallService() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	target := filepath.Join(os.Getenv("ProgramFiles"), "CloudPrint", "cloudprint-agent.exe")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	if self != target {
		data, err := os.ReadFile(self)
		if err != nil {
			return fmt.Errorf("read binary: %w", err)
		}
		if err := os.WriteFile(target, data, 0755); err != nil {
			return fmt.Errorf("copy binary: %w", err)
		}
	}

	args := []string{
		"create", "CloudPrintAgent",
		"binPath=", target + " run",
		"DisplayName=", "CloudPrint Agent",
		"start=", "auto",
		"Description=", "CloudPrint local printer agent",
	}
	out, err := exec.Command("sc", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc create: %w — %s", err, out)
	}

	fmt.Println("Service installed. Start with: sc start CloudPrintAgent")
	return nil
}

// UninstallService removes the Windows service.
func UninstallService() error {
	exec.Command("sc", "stop", "CloudPrintAgent").Run()
	out, err := exec.Command("sc", "delete", "CloudPrintAgent").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc delete: %w — %s", err, out)
	}
	fmt.Println("Service uninstalled.")
	return nil
}

// StartService starts the Windows service.
func StartService() error {
	out, err := exec.Command("sc", "start", "CloudPrintAgent").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc start: %w — %s", err, out)
	}
	fmt.Println("Service started.")
	return nil
}

// StopService stops the Windows service.
func StopService() error {
	out, err := exec.Command("sc", "stop", "CloudPrintAgent").CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc stop: %w — %s", err, out)
	}
	fmt.Println("Service stopped.")
	return nil
}
