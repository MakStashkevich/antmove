package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

var stateDirectoryOverride string

func stateDirectory() string {
	if stateDirectoryOverride != "" {
		return stateDirectoryOverride
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	}
	return filepath.Join(base, "antmove")
}

func operationsDirectory() string {
	return filepath.Join(stateDirectory(), "operations")
}

func ensureStateDirectory() error {
	return os.MkdirAll(operationsDirectory(), 0o700)
}

func loadOperations() ([]Operation, error) {
	entries, err := os.ReadDir(operationsDirectory())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []Operation
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(operationsDirectory(), entry.Name()))
		if err != nil {
			return nil, err
		}
		var op Operation
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, fmt.Errorf("read operation %s: %w", entry.Name(), err)
		}
		result = append(result, op)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func saveOperation(op Operation) error {
	if err := ensureStateDirectory(); err != nil {
		return err
	}
	op.UpdatedAt = time.Now()
	return atomicJSON(filepath.Join(operationsDirectory(), op.ID+".json"), op)
}

func removeOperation(op Operation) error {
	return os.Remove(filepath.Join(operationsDirectory(), op.ID+".json"))
}

func atomicJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".antmove-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func acquireLock() (func(), error) {
	if err := ensureStateDirectory(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(stateDirectory(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("another antmove operation is running")
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
