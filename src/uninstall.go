package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const uninstallConfirmation = "DELETE antmove and its history"
const managedPathBlock = "\n# antmove: keep the am command available in this shell\nexport PATH=\"$HOME/.local/bin:$PATH\"\n"

func runUninstall() error {
	fmt.Println("WARNING: This permanently removes antmove, the am alias, and antmove's operation history.")
	fmt.Println("Moved apps and data will NOT be restored. Their original paths remain symbolic links; keep their disks connected or restore them before uninstalling.")
	if err := confirmUninstallUsing(confirm, readLine); err != nil {
		return err
	}

	if err := performUninstall(); err != nil {
		return err
	}
	fmt.Println("Removed antmove, am, and antmove data. You can delete the extracted download manually.")
	return nil
}

func performUninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unlock, err := acquireLock()
	if err != nil {
		return fmt.Errorf("cannot uninstall while antmove is busy: %w", err)
	}
	defer unlock()
	binDirectory, err := filepath.Abs(installBinDirectory(home))
	if err != nil {
		return err
	}
	if err := uninstallAntmove(home, binDirectory, stateDirectory()); err != nil {
		return err
	}
	return nil
}

func confirmUninstallUsing(confirmFn func(string) bool, readFn func() string) error {
	if !confirmFn("First confirmation: uninstall antmove? [y/N] ") {
		return errors.New("uninstall cancelled")
	}
	fmt.Printf("Second confirmation: type %q: ", uninstallConfirmation)
	if answer := readFn(); answer == uninstallConfirmation {
		return nil
	}
	return errors.New("confirmation did not match; uninstall cancelled")
}

func uninstallAntmove(home, binDirectory, stateDir string) error {
	if home == "" || binDirectory == "" || filepath.Clean(binDirectory) == filepath.Clean(home) || filepath.Dir(filepath.Clean(binDirectory)) == filepath.Clean(binDirectory) {
		return errors.New("refusing to remove an unsafe installation directory")
	}
	if stateDir != "" {
		cleanState := filepath.Clean(stateDir)
		if filepath.Base(cleanState) != "antmove" || cleanState == filepath.Clean(home) || filepath.Dir(cleanState) == cleanState {
			return errors.New("refusing to remove an unsafe antmove data directory")
		}
	}
	aliasPath := filepath.Join(binDirectory, "am")
	if target, err := os.Readlink(aliasPath); err == nil {
		if target != "antmove" {
			return fmt.Errorf("refusing to remove unrelated am alias at %s", aliasPath)
		}
	} else if !os.IsNotExist(err) {
		if _, statErr := os.Lstat(aliasPath); statErr == nil {
			return fmt.Errorf("refusing to remove unrelated am command at %s", aliasPath)
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("inspect am command: %w", statErr)
		}
	}
	binaryPath := filepath.Join(binDirectory, "antmove")
	if info, err := os.Lstat(binaryPath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-regular file at %s", binaryPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect antmove binary: %w", err)
	}

	if err := removeManagedPathBlock(home); err != nil {
		return err
	}
	if stateDir != "" {
		if err := os.RemoveAll(stateDir); err != nil {
			return fmt.Errorf("remove antmove data %s: %w", stateDir, err)
		}
	}
	if target, err := os.Readlink(aliasPath); err == nil {
		if target == "antmove" {
			if err := os.Remove(aliasPath); err != nil {
				return fmt.Errorf("remove am alias: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect am alias: %w", err)
	}

	info, err := os.Lstat(binaryPath)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-regular file at %s", binaryPath)
		}
		if err := os.Remove(binaryPath); err != nil {
			return fmt.Errorf("remove antmove binary: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect antmove binary: %w", err)
	}
	return nil
}

func removeManagedPathBlock(home string) error {
	for _, profile := range shellProfilePaths(home) {
		contents, err := os.ReadFile(profile)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read shell startup file %s: %w", profile, err)
		}
		updated := strings.Replace(string(contents), managedPathBlock, "\n", 1)
		if updated == string(contents) {
			continue
		}
		if err := os.WriteFile(profile, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("update shell startup file %s: %w", profile, err)
		}
	}
	return nil
}

func shellProfilePaths(home string) []string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return []string{filepath.Join(home, ".zshrc"), filepath.Join(home, ".zprofile")}
	case "bash":
		return []string{filepath.Join(home, ".bash_profile"), filepath.Join(home, ".bashrc")}
	default:
		return []string{filepath.Join(home, ".profile")}
	}
}
