//go:build !darwin

package main

import "fmt"

func launchdInstall(_ string) error {
	return fmt.Errorf("launchd is only supported on macOS")
}

func launchdUninstall() error {
	return fmt.Errorf("launchd is only supported on macOS")
}
