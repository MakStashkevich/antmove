package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func installSelf() (installed, pathConfigChanged bool, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, false, err
	}
	binDirectory := installBinDirectory(home)
	binDirectory, err = filepath.Abs(binDirectory)
	if err != nil {
		return false, false, err
	}
	if err := os.MkdirAll(binDirectory, 0o755); err != nil {
		return false, false, fmt.Errorf("create %s: %w", binDirectory, err)
	}

	executable, err := os.Executable()
	if err != nil {
		return false, false, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return false, false, err
	}
	installedPath := filepath.Join(binDirectory, "antmove")
	if filepath.Clean(executable) != filepath.Clean(installedPath) {
		if err := copyExecutable(executable, installedPath); err != nil {
			return false, false, err
		}
		installed = true
	}
	if err := ensureAlias(filepath.Join(binDirectory, "am")); err != nil {
		return installed, false, err
	}
	if filepath.Clean(binDirectory) == filepath.Clean(filepath.Join(home, ".local", "bin")) {
		pathConfigChanged, err = ensureShellPath(home, binDirectory)
		if err != nil {
			return installed, false, err
		}
	}
	return installed, pathConfigChanged, nil
}

func installBinDirectory(home string) string {
	if directory := os.Getenv("ANTMOVE_INSTALL_DIR"); directory != "" {
		return directory
	}
	return filepath.Join(home, ".local", "bin")
}

func installDirectoryLabel() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return installBinDirectory("")
	}
	directory := installBinDirectory(home)
	if absolute, err := filepath.Abs(directory); err == nil {
		directory = absolute
	}
	if relative, err := filepath.Rel(home, directory); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.Join("~", relative)
	}
	return directory
}

func copyExecutable(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".antmove-install-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o755); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := io.Copy(temporary, input); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("install antmove to %s: %w", destination, err)
	}
	return nil
}

func ensureAlias(aliasPath string) error {
	info, err := os.Lstat(aliasPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(aliasPath)
			if readErr == nil && target == "antmove" {
				return nil
			}
		}
		return fmt.Errorf("%s already exists and is not antmove's am alias; leaving it untouched", aliasPath)
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink("antmove", aliasPath); err != nil {
		return fmt.Errorf("create command alias %s: %w", aliasPath, err)
	}
	return nil
}

func ensureShellPath(home, binDirectory string) (bool, error) {
	profiles := shellProfilePaths(home)
	target := profiles[0]
	for _, profile := range profiles {
		data, err := os.ReadFile(profile)
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if strings.Contains(string(data), binDirectory) || strings.Contains(string(data), "$HOME/.local/bin") || strings.Contains(string(data), "${HOME}/.local/bin") {
			return false, nil
		}
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("update shell startup file %s: %w", target, err)
	}
	defer file.Close()
	if _, err := file.WriteString(managedPathBlock); err != nil {
		return false, fmt.Errorf("update shell startup file %s: %w", target, err)
	}
	return true, nil
}

func autoInstallOnFirstRun(command string) bool {
	return command != "install" && command != "uninstall"
}
