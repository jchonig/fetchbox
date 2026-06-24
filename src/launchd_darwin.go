//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const launchdLabel = "net.honig.fetchbox"

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Executable}}</string>
		<string>--daemon</string>
		<string>--config</string>
		<string>{{.Config}}</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogFile}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogFile}}</string>
</dict>
</plist>
`))

func launchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func launchdInstall(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	plistPath, err := launchdPlistPath()
	if err != nil {
		return fmt.Errorf("plist path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	logDir := filepath.Join(home, "Library", "Logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create Logs dir: %w", err)
	}

	f, err := os.Create(plistPath)
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer f.Close()

	if err := plistTmpl.Execute(f, map[string]string{
		"Label":      launchdLabel,
		"Executable": exe,
		"Config":     configPath,
		"LogFile":    filepath.Join(logDir, "fetchbox.log"),
	}); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	f.Close()

	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d", uid)

	// bootout first so re-install and post-upgrade installs work cleanly
	exec.Command("launchctl", "bootout", target, plistPath).Run() //nolint:errcheck

	out, err := exec.Command("launchctl", "bootstrap", target, plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %w\n%s", err, out)
	}

	fmt.Printf("Installed %s\nPlist: %s\nLog:   %s\n",
		launchdLabel, plistPath, filepath.Join(logDir, "fetchbox.log"))
	return nil
}

func launchdUninstall() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return fmt.Errorf("plist path: %w", err)
	}

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Printf("%s is not installed\n", launchdLabel)
		return nil
	}

	uid := os.Getuid()
	out, err := exec.Command("launchctl", "bootout",
		fmt.Sprintf("gui/%d", uid), plistPath).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: launchctl bootout: %v\n%s\n", err, out)
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Printf("Uninstalled %s\n", launchdLabel)
	return nil
}
