//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveDefaultConfigPath() string {
	home, _ := os.UserHomeDir()

	// 1. ~/Library/Application Support/codeseek/config.yml (macOS preferred)
	if home != "" {
		c := filepath.Join(home, "Library", "Application Support", "codeseek", "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 2. ~/.config/codeseek/config.yml (XDG)
	if home != "" {
		c := filepath.Join(home, ".config", "codeseek", "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 3. Next to the executable
	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 4. Current directory
	if _, err := os.Stat("config.yml"); err == nil {
		abs, _ := filepath.Abs("config.yml")
		return abs
	}
	return ""
}

// isSystemPath checks if path is under a protected system directory.
func isSystemPath(path string) bool {
	return strings.HasPrefix(path, "/Applications/") ||
		strings.HasPrefix(path, "/System/") ||
		(path == "/Applications" || path == "/System" || path == "/Library")
}

// checkCodexProcess checks if any codex process is running (macOS).
func checkCodexProcess() bool {
	cmd := exec.Command("pgrep", "-f", "[Cc]odex")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func configPathNotFoundMessage() string {
	return "未找到配置文件，请将 config.yml 放在程序同目录或 ~/Library/Application Support/codeseek/"
}

// defaultConfigPath returns the path where config should be created on first launch.
func defaultConfigPath() (string, error) {
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, "Library", "Application Support", "codeseek", "config.yml"), nil
	}
	return "", errors.New("无法确定用户目录")
}
