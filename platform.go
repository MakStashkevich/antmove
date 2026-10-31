package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func commandOutput(path string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	return cmd.Output()
}

func plistJSON(plist []byte) ([]byte, error) {
	cmd := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(plist)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plutil: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func plistMap(data []byte) (map[string]any, error) {
	jsonData, err := plistJSON(data)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func stringProperty(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func boolProperty(values map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := values[key].(bool); ok {
			return value, true
		}
		if value, ok := values[key].(float64); ok {
			return value != 0, true
		}
	}
	return false, false
}

func int64Property(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := values[key].(float64); ok && value >= 0 {
			return int64(value)
		}
	}
	return 0
}
