package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed assets/ant.png
var antFolderIcon []byte

func setAntmoveFolderIcon(mountPath string) error {
	root := filepath.Join(mountPath, "antmove")
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect antmove folder: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("antmove folder is not a real directory")
	}
	if _, err := os.Lstat(filepath.Join(root, "Icon\r")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect antmove folder icon: %w", err)
	}

	iconFile, err := os.CreateTemp("", "antmove-folder-icon-*.png")
	if err != nil {
		return fmt.Errorf("prepare antmove folder icon: %w", err)
	}
	iconPath := iconFile.Name()
	defer os.Remove(iconPath)
	if _, err := iconFile.Write(antFolderIcon); err != nil {
		_ = iconFile.Close()
		return fmt.Errorf("write antmove folder icon: %w", err)
	}
	if err := iconFile.Close(); err != nil {
		return fmt.Errorf("close antmove folder icon: %w", err)
	}

	imagePathArg, _ := json.Marshal(iconPath)
	rootPathArg, _ := json.Marshal(root)
	script := fmt.Sprintf(`ObjC.import("AppKit");
var image = $.NSImage.alloc.initWithContentsOfFile(%s);
if (!image) throw new Error("could not load the antmove icon image");
var didSet = $.NSWorkspace.sharedWorkspace.setIconForFileOptions(image, %s, 0);
if (!didSet) throw new Error("Finder could not set the antmove folder icon");`, imagePathArg, rootPathArg)
	output, err := exec.Command("/usr/bin/osascript", "-l", "JavaScript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set antmove folder icon: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Lstat(filepath.Join(root, "Icon\r")); err != nil {
		return fmt.Errorf("Finder did not create the antmove folder icon file: %w", err)
	}
	return nil
}
