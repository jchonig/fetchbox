//go:build !darwin

package main

import (
	"fmt"
	"os"
)

// getSecret returns the value of the env var named by envKey.
// Keychain is not supported on non-darwin platforms.
func getSecret(envKey, service, account string) (string, error) {
	if envKey != "" {
		if val := os.Getenv(envKey); val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("secret %s/%s not found: set env var %q", service, account, envKey)
}
