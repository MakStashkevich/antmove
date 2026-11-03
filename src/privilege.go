package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func directoryNeedsPrivilege(path string) bool {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		if _, err := os.Stat(current); err == nil {
			return unix.Access(current, unix.W_OK) != nil
		} else if !os.IsNotExist(err) {
			return unix.Access(current, unix.W_OK) != nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
	}
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

func dittoCopyProgress(source, destination string, privileged bool, progress func(int64)) ([]byte, error) {
	if progress == nil {
		return dittoCopy(source, destination, privileged)
	}
	args := []string{"-V", "--rsrc", "--extattr", "--acl", source, destination}
	command := exec.Command("/usr/bin/ditto", args...)
	if privileged {
		command = exec.Command("/usr/bin/sudo", append([]string{"/usr/bin/ditto"}, args...)...)
		command.Stdin = os.Stdin
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if err := command.Start(); err != nil {
		return nil, err
	}
	paths := make(chan string)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer close(paths)
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = stderr.WriteString(line + "\n")
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "copying file ") || !strings.HasSuffix(line, " ...") {
				continue
			}
			rel := strings.TrimSuffix(strings.TrimPrefix(line, "copying file "), " ...")
			rel = strings.TrimPrefix(rel, "./")
			paths <- rel
		}
	}()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	var completed, currentTotal, currentDone int64
	current := ""
	pollCurrent := func() {
		if current == "" {
			return
		}
		info, statErr := os.Stat(filepath.Join(destination, current))
		if statErr == nil && info.Mode().IsRegular() {
			size := allocatedFileBytes(info)
			if size > currentTotal {
				size = currentTotal
			}
			if size > currentDone {
				currentDone = size
				progress(completed + currentDone)
			}
		}
	}
	for paths != nil {
		select {
		case rel, ok := <-paths:
			if !ok {
				paths = nil
				break
			}
			pollCurrent()
			completed += currentTotal
			current, currentTotal, currentDone = "", 0, 0
			info, statErr := os.Lstat(filepath.Join(source, rel))
			if statErr == nil && info.Mode().IsRegular() {
				current, currentTotal = rel, allocatedFileBytes(info)
				pollCurrent()
			}
		case <-ticker.C:
			pollCurrent()
		}
	}
	pollCurrent()
	completed += currentTotal
	progress(completed)
	err = command.Wait()
	<-finished
	if err != nil {
		return append(stdout.Bytes(), stderr.Bytes()...), err
	}
	return append(stdout.Bytes(), stderr.Bytes()...), nil
}

func allocatedFileBytes(info os.FileInfo) int64 {
	if raw, ok := info.Sys().(*syscall.Stat_t); ok && raw.Blocks > 0 {
		return raw.Blocks * 512
	}
	return info.Size()
}
