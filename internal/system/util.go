package system

import (
	"fmt"
	"os/exec"
)

func reloadService(name string) error {
	if err := exec.Command("rc-service", name, "reload").Run(); err != nil {
		return fmt.Errorf("rc-service %s reload: %w", name, err)
	}
	return nil
}

func restartService(name string) error {
	if err := exec.Command("rc-service", name, "restart").Run(); err != nil {
		return fmt.Errorf("rc-service %s restart: %w", name, err)
	}
	return nil
}
