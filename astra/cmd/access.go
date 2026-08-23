package main

// access.go contains the user-facing filesystem preflight for `astra connect`.
// Astra's scope registry records application authority, but it cannot grant or
// bypass macOS TCC privacy permissions. This preflight catches that distinction
// before an agent starts a run and gives the user a recoverable UI flow.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const macOSPrivacySettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

// ensureWorkspaceAccess waits until the connected directory can actually be
// read by this process. The scope ledger is checked later by action execution;
// this check verifies the operating system boundary first.
func ensureWorkspaceAccess(path string) bool {
	for {
		err := checkDirectoryAccess(path)
		if err == nil {
			return true
		}
		if !isFilesystemPermissionError(err) {
			fmt.Printf("%s\n", accessErrorText(path, err))
			return false
		}

		fmt.Printf("\nAstra needs operating-system permission to read this workspace:\n  %s\n\n", path)
		fmt.Println("This is different from Astra's approved scope. macOS must approve the application that launched Astra.")
		if runtime.GOOS == "darwin" {
			fmt.Println("[o] Open macOS Privacy Settings   [r] Retry access   [q] Quit")
		} else {
			fmt.Println("[r] Retry access   [q] Quit")
		}

		if !isInteractiveTerminal() {
			fmt.Println("Non-interactive terminal detected. Grant access, then run `astra connect` again.")
			return false
		}
		choice := readAccessChoice()
		switch choice {
		case "o":
			if err := openMacOSPrivacySettings(); err != nil {
				fmt.Printf("Could not open macOS Privacy Settings: %v\n", err)
			} else {
				fmt.Println("Privacy Settings opened. Enable access for Terminal, iTerm, VS Code, or the application launching Astra, then choose [r].")
			}
		case "r":
			continue
		default:
			fmt.Println("Astra did not start because workspace access is unavailable.")
			return false
		}
	}
}

func checkDirectoryAccess(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil // An empty directory is readable.
	}
	return err
}

func isFilesystemPermissionError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return errors.Is(err, os.ErrPermission) || strings.Contains(text, "operation not permitted") || strings.Contains(text, "permission denied") || strings.Contains(text, "access denied")
}

func accessErrorText(path string, err error) string {
	return fmt.Sprintf("Astra cannot access workspace %q: %v", path, err)
}

func isInteractiveTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func readAccessChoice() string {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "q"
	}
	return strings.ToLower(strings.TrimSpace(line))
}

func openMacOSPrivacySettings() error {
	if runtime.GOOS != "darwin" {
		return errors.New("macOS Privacy Settings are only available on Darwin")
	}
	return exec.Command("open", macOSPrivacySettingsURL).Run()
}
