//go:build !windows && !darwin

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

	// 1. ~/.config/codeseek/config.yml (XDG)
	if home != "" {
		c := filepath.Join(home, ".config", "codeseek", "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 2. Next to the executable
	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 3. Current directory
	if _, err := os.Stat("config.yml"); err == nil {
		abs, _ := filepath.Abs("config.yml")
		return abs
	}
	return ""
}

// isSystemPath checks if path is under a protected system directory.
func isSystemPath(path string) bool {
	return strings.HasPrefix(path, "/usr/") ||
		strings.HasPrefix(path, "/etc/") ||
		strings.HasPrefix(path, "/boot/")
}

// checkCodexProcess checks if any codex process is running (Unix).
func checkCodexProcess() bool {
	cmd := exec.Command("pgrep", "-f", "[Cc]odex")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func configPathNotFoundMessage() string {
	return "未找到配置文件，请将 config.yml 放在程序同目录或 ~/.config/codeseek/"
}

// defaultConfigPath returns the path where config should be created on first launch.
func defaultConfigPath() (string, error) {
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".config", "codeseek", "config.yml"), nil
	}
	return "", errors.New("无法确定用户目录")
}
