//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveDefaultConfigPath() string {
	// 1. %APPDATA%/CodeSeek/config.yml (writable, preferred)
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		c := filepath.Join(appdata, "codeseek", "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// 2. Next to the exe (read-only template on first run)
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
	// 4. %USERPROFILE%/.config/codeseek/config.yml
	if home := os.Getenv("USERPROFILE"); home != "" {
		c := filepath.Join(home, ".config", "codeseek", "config.yml")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// isSystemPath checks if path is under a protected system directory.
func isSystemPath(path string) bool {
	lower := strings.ToLower(path)
	root := strings.ToLower(os.Getenv("SystemDrive"))
	if root == "" {
		root = "c:"
	}
	return strings.HasPrefix(lower, root+"\\program files") ||
		strings.HasPrefix(lower, root+"\\program files (x86)") ||
		strings.HasPrefix(lower, root+"\\windows")
}

// checkCodexProcess checks if any codex.exe process is running (Windows).
func checkCodexProcess() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq codex.exe", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "codex.exe")
}

func configPathNotFoundMessage() string {
	return "未找到配置文件，请将 config.yml 放在程序同目录或 %APPDATA%/codeseek/"
}

// defaultConfigPath returns the path where config should be created on first launch.
func defaultConfigPath() (string, error) {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "codeseek", "config.yml"), nil
	}
	return "", errors.New("无法确定 APPDATA 目录")
}
