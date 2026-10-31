package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func directoryNeedsPrivilege(path string) bool {
	return unix.Access(path, unix.W_OK) != nil
}

func ensurePrivilege(path string, needed bool) error {
	if !needed {
		return nil
	}
	command := exec.Command("/usr/bin/sudo", "-v")
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("administrator authorization is required to modify %s: %w", path, err)
	}
	return nil
}

func runPrivileged(executable string, args ...string) error {
	commandArgs := append([]string{executable}, args...)
	command := exec.Command("/usr/bin/sudo", commandArgs...)
	command.Stdin = os.Stdin
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("administrator command %s failed: %w: %s", filepath.Base(executable), err, output)
	}
	return nil
}

func renamePath(oldPath, newPath string, privileged bool) error {
	if !privileged {
		return os.Rename(oldPath, newPath)
	}
	return runPrivileged("/bin/mv", oldPath, newPath)
}

func symlinkPath(target, linkPath string, privileged bool) error {
	if !privileged {
		return os.Symlink(target, linkPath)
	}
	return runPrivileged("/bin/ln", "-s", target, linkPath)
}

func removeAllPath(path string, privileged bool) error {
	if !privileged {
		return os.RemoveAll(path)
	}
	return runPrivileged("/bin/rm", "-rf", path)
}

func dittoCopy(source, destination string, privileged bool) ([]byte, error) {
	args := []string{"--rsrc", "--extattr", "--acl", source, destination}
	command := exec.Command("/usr/bin/ditto", args...)
	if privileged {
		command = exec.Command("/usr/bin/sudo", append([]string{"/usr/bin/ditto"}, args...)...)
		command.Stdin = os.Stdin
	}
	return command.CombinedOutput()
}
