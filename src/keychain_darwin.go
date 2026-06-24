//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// getSecret returns a secret. Priority:
//  1. env var named by envKey (if non-empty and set)
//  2. macOS Keychain (via the security CLI)
//  3. interactive prompt (TTY only) — stores result in Keychain
func getSecret(envKey, service, account string) (string, error) {
	if envKey != "" {
		if val := os.Getenv(envKey); val != "" {
			return val, nil
		}
	}

	out, err := exec.Command("security", "find-generic-password",
		"-a", account, "-s", service, "-w").Output()
	if err == nil {
		return strings.TrimRight(string(out), "\n"), nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("secret %s/%s not found in Keychain and stdin is not a terminal", service, account)
	}

	fmt.Fprintf(os.Stderr, "Enter secret for %s (%s): ", service, account)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	val := string(pw)
	if val == "" {
		return "", fmt.Errorf("no secret provided for %s/%s", service, account)
	}

	if out, err := exec.Command("security", "add-generic-password",
		"-U", "-a", account, "-s", service, "-w", val).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not store in Keychain: %v (%s)\n", err, bytes.TrimSpace(out))
	}

	return val, nil
}
