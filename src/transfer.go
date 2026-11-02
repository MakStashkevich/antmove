package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type manifestEntry struct {
	Kind   string `json:"kind"`
	Size   int64  `json:"size,omitempty"`
	Target string `json:"target,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

func moveItem(item Item, volume Volume, dryRun bool) (Operation, error) {
	if item.Status == StatusUnavailable || item.Status == StatusMoved || item.Status == StatusDiskOffline {
		return Operation{}, fmt.Errorf("%s cannot be moved: %s", item.Name, item.Reason)
	}
	if volume.Rating == RatingUnavailable || !volume.Physical || volume.Network || !volume.Writable {
		return Operation{}, fmt.Errorf("destination disk is unavailable for transfers")
	}
	if item.Category == CategoryContainer {
		return Operation{}, fmt.Errorf("container storage must be moved with the engine's supported settings")
	}
	if hasSymlinkAncestor(item.SourcePath) {
		return Operation{}, fmt.Errorf("source path or one of its parents is a symbolic link")
	}
	sourceInfo, err := os.Lstat(item.SourcePath)
	sourceMissing := os.IsNotExist(err)
	canCreateMissingSource := item.CreateWhenMissing && item.SourceFingerprint == ""
	if sourceMissing && !canCreateMissingSource {
		return Operation{}, fmt.Errorf("source no longer exists; scan again before moving it")
	}
	if !sourceMissing && canCreateMissingSource {
		return Operation{}, fmt.Errorf("source appeared after the scan; scan again before moving it")
	}
	if (err != nil && !sourceMissing) || (!sourceMissing && sourceInfo.Mode()&os.ModeSymlink != 0) {
		return Operation{}, fmt.Errorf("source is no longer an ordinary file or directory")
	}
	if !sourceMissing && item.SourceFingerprint != "" && fingerprint(sourceInfo) != item.SourceFingerprint {
		return Operation{}, fmt.Errorf("source changed since the last scan; scan again before moving it")
	}
	var currentSize int64
	if !sourceMissing {
		currentSize, err = allocatedSize(item.SourcePath)
		if err != nil {
			return Operation{}, fmt.Errorf("measure source before transfer: %w", err)
		}
	}
	needed := requiredCapacity(currentSize)
	if item.CreateWhenMissing && currentSize == 0 {
		needed = 0
	}
	currentVolumes, err := scanVolumes()
	if err != nil {
		return Operation{}, err
	}
	current := findVolume(currentVolumes, volume.UUID, volume.MountPath)
	if current == nil {
		return Operation{}, fmt.Errorf("destination disk was disconnected or changed")
	}
	if current.FreeBytes < needed {
		return Operation{}, fmt.Errorf("not enough free space: need %s including safety margin, have %s", formatBytes(needed), formatBytes(current.FreeBytes))
	}
	if dryRun {
		return Operation{ItemID: item.ID, Name: item.Name, SourcePath: item.SourcePath,
			DestinationPath: destinationPath(item.SourcePath, *current), VolumeUUID: current.UUID,
			VolumeName: current.Name, SizeBytes: currentSize, Phase: "preview"}, nil
	}
	privileged := directoryNeedsPrivilege(filepath.Dir(item.SourcePath))
	if err := ensurePrivilege(filepath.Dir(item.SourcePath), privileged); err != nil {
		return Operation{}, err
	}
	if err := writableProbe(current.MountPath); err != nil {
		return Operation{}, fmt.Errorf("destination disk is not writable: %w", err)
	}
	finalPath := destinationPath(item.SourcePath, *current)
	if _, err := os.Lstat(finalPath); err == nil {
		return Operation{}, fmt.Errorf("destination already exists: %s", finalPath)
	}
	if err := ensureSafeDestinationParent(current.MountPath, filepath.Dir(finalPath)); err != nil {
		return Operation{}, err
	}
	// Folder icons are cosmetic; some external filesystems cannot store them.
	// Keep the transfer usable even when Finder cannot persist this metadata.
	_ = setAntmoveFolderIcon(current.MountPath)
	if err := diskStillMounted(*current); err != nil {
		return Operation{}, err
	}
	if activity, ok := activeActivityFor(item); ok {
		return Operation{}, activityError(item, activity)
	}
	id := randomID()
	staging := filepath.Join(filepath.Dir(finalPath), ".antmove-staging-"+id)
	backup := filepath.Join(filepath.Dir(item.SourcePath), ".antmove-backup-"+id)
	op := Operation{
		ID: id, ItemID: item.ID, Name: item.Name, Category: item.Category,
		SourcePath: item.SourcePath, DestinationPath: finalPath, VolumeUUID: current.UUID, VolumeMountPath: current.MountPath,
		VolumeName: current.Name, SizeBytes: currentSize, Phase: "preparing",
		CreatedAt: time.Now(), StagingPath: staging, BackupPath: backup,
	}
	if err := saveOperation(op); err != nil {
		return Operation{}, err
	}
	if sourceMissing {
		if hasSymlinkAncestor(item.SourcePath) {
			op.Phase, op.Error = "failed", "source path or one of its parents became a symbolic link"
			_ = saveOperation(op)
			return op, fmt.Errorf("source path or one of its parents became a symbolic link")
		}
		if _, err := os.Lstat(item.SourcePath); err == nil {
			op.Phase, op.Error = "failed", "source appeared after the scan; no files were moved"
			_ = saveOperation(op)
			return op, fmt.Errorf("source appeared after the scan; scan again before moving it")
		} else if !os.IsNotExist(err) {
			op.Phase, op.Error = "failed", err.Error()
			_ = saveOperation(op)
			return op, fmt.Errorf("check source path before creating it: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(item.SourcePath), 0o755); err != nil {
			op.Phase, op.Error = "failed", err.Error()
			_ = saveOperation(op)
			return op, fmt.Errorf("create source parent directory: %w", err)
		}
		if err := os.Mkdir(item.SourcePath, 0o755); err != nil {
			op.Phase, op.Error = "failed", err.Error()
			_ = saveOperation(op)
			return op, fmt.Errorf("create empty source directory: %w", err)
		}
		sourceInfo, err = os.Lstat(item.SourcePath)
		if err != nil {
			op.Phase, op.Error = "failed", err.Error()
			_ = saveOperation(op)
			return op, fmt.Errorf("inspect new source directory: %w", err)
		}
		item.SourceFingerprint = fingerprint(sourceInfo)
	}

	op.Phase = "copying"
	if err := saveOperation(op); err != nil {
		return Operation{}, err
	}
	if output, err := dittoCopy(item.SourcePath, staging, false); err != nil {
		op.Phase, op.Error = "failed", strings.TrimSpace(string(output))
		_ = saveOperation(op)
		_ = os.RemoveAll(staging)
		return op, fmt.Errorf("copy failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	op.Phase = "verifying"
	if err := saveOperation(op); err != nil {
		return op, err
	}
	if err := verifyTree(item.SourcePath, staging); err != nil {
		op.Phase, op.Error = "failed", err.Error()
		_ = saveOperation(op)
		_ = os.RemoveAll(staging)
		return op, fmt.Errorf("verification failed; original was not changed: %w", err)
	}
	latestSourceInfo, err := os.Lstat(item.SourcePath)
	if err != nil || (item.SourceFingerprint != "" && fingerprint(latestSourceInfo) != item.SourceFingerprint) {
		_ = os.RemoveAll(staging)
		_ = os.RemoveAll(finalPath)
		op.Phase, op.Error = "failed", "source changed during copy; original was not moved"
		_ = saveOperation(op)
		return op, fmt.Errorf("source changed during copy; original was not moved")
	}
	op.Phase = "finalizing"
	if err := saveOperation(op); err != nil {
		_ = os.RemoveAll(staging)
		return Operation{}, err
	}
	if err := diskStillMounted(*current); err != nil {
		op.Error = err.Error()
		_ = saveOperation(op)
		return op, err
	}
	if err := os.Rename(staging, finalPath); err != nil {
		op.Phase, op.Error = "failed", err.Error()
		_ = saveOperation(op)
		_ = os.RemoveAll(staging)
		return op, fmt.Errorf("finalize destination: %w", err)
	}
	op.StagingPath = ""
	op.Phase = "linking"
	if err := saveOperation(op); err != nil {
		_ = os.RemoveAll(finalPath)
		return op, err
	}
	if err := renamePath(item.SourcePath, backup, privileged); err != nil {
		op.Phase, op.Error = "failed", err.Error()
		_ = saveOperation(op)
		_ = os.RemoveAll(finalPath)
		return op, fmt.Errorf("stage original for linking: %w", err)
	}
	if err := symlinkPath(finalPath, item.SourcePath, privileged); err != nil {
		rollbackErr := renamePath(backup, item.SourcePath, privileged)
		op.Phase, op.Error = "failed", err.Error()
		if rollbackErr != nil {
			op.Phase, op.Error = "linking", "could not restore original after symlink failure: "+rollbackErr.Error()
		}
		_ = saveOperation(op)
		if rollbackErr == nil {
			_ = os.RemoveAll(finalPath)
		}
		return op, fmt.Errorf("create symbolic link: %w", err)
	}
	linkTarget, err := os.Readlink(item.SourcePath)
	if err != nil || filepath.Clean(linkTarget) != filepath.Clean(finalPath) {
		_ = removeAllPath(item.SourcePath, privileged)
		rollbackErr := renamePath(backup, item.SourcePath, privileged)
		op.Phase, op.Error = "failed", "could not verify the new symbolic link"
		if rollbackErr != nil {
			op.Phase, op.Error = "linking", "could not verify the new symbolic link or restore original: "+rollbackErr.Error()
		} else {
			_ = os.RemoveAll(finalPath)
		}
		_ = saveOperation(op)
		return op, fmt.Errorf("could not verify symbolic link; original restored")
	}
	op.Phase = "cleanup_pending"
	if err := saveOperation(op); err != nil {
		return op, err
	}
	if err := removeAllPath(backup, privileged); err != nil {
		op.Error = "linked successfully; backup cleanup pending: " + err.Error()
		_ = saveOperation(op)
		return op, fmt.Errorf("data moved, but original backup cleanup is pending: %w", err)
	}
	op.BackupPath = ""
	op.Phase, op.Error = "completed", ""
	if err := saveOperation(op); err != nil {
		return op, err
	}
	return op, nil
}

func restoreOperation(op Operation) error {
	if op.Phase != "completed" && op.Phase != "cleanup_pending" {
		return fmt.Errorf("%s is not in a restorable state (%s)", op.Name, op.Phase)
	}
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	volume := findOperationVolume(volumes, op)
	if volume == nil {
		return fmt.Errorf("destination disk %q is not connected", op.VolumeName)
	}
	if err := activeProcessFor(Item{Name: op.Name, SourcePath: op.SourcePath, Category: op.Category, Owner: op.Name}); err != nil {
		return err
	}
	if hasSymlinkParent(op.SourcePath) {
		return fmt.Errorf("original parent path is a symbolic link; refusing to restore through it")
	}
	if err := diskStillMounted(*volume); err != nil {
		return err
	}
	target, err := os.Readlink(op.SourcePath)
	if err != nil || filepath.Clean(target) != filepath.Clean(op.DestinationPath) {
		return fmt.Errorf("source link no longer points to antmove's destination; refusing to overwrite it")
	}
	if _, err := os.Stat(op.DestinationPath); err != nil {
		return fmt.Errorf("moved data is unavailable: %w", err)
	}
	currentSize, err := allocatedSize(op.DestinationPath)
	if err != nil {
		return fmt.Errorf("measure moved data before restore: %w", err)
	}
	op.SizeBytes = currentSize
	parent := filepath.Dir(op.SourcePath)
	var stats unix.Statfs_t
	if err := unix.Statfs(parent, &stats); err != nil {
		return fmt.Errorf("check local free space: %w", err)
	}
	freeBytes := int64(stats.Bavail * uint64(stats.Bsize))
	needed := requiredCapacity(currentSize)
	if currentSize == 0 {
		needed = 0
	}
	if freeBytes < needed {
		return fmt.Errorf("not enough local free space to restore %s (need %s, have %s)", op.Name, formatBytes(needed), formatBytes(freeBytes))
	}
	stage := filepath.Join(parent, ".antmove-restore-"+op.ID)
	if _, err := os.Lstat(stage); err == nil {
		return fmt.Errorf("restore staging path already exists: %s", stage)
	}
	privileged := directoryNeedsPrivilege(parent)
	if err := ensurePrivilege(parent, privileged); err != nil {
		return err
	}
	linkBackup := op.SourcePath + ".antmove-link-" + op.ID
	op.Phase, op.StagingPath, op.BackupPath, op.Error = "restoring", stage, linkBackup, ""
	if err := saveOperation(op); err != nil {
		return err
	}
	if output, err := dittoCopy(op.DestinationPath, stage, privileged); err != nil {
		_ = removeAllPath(stage, privileged)
		op.Phase, op.StagingPath, op.BackupPath, op.Error = "completed", "", "", ""
		_ = saveOperation(op)
		return fmt.Errorf("restore copy failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := verifyTree(op.DestinationPath, stage); err != nil {
		_ = removeAllPath(stage, privileged)
		op.Phase, op.StagingPath, op.BackupPath, op.Error = "completed", "", "", ""
		_ = saveOperation(op)
		return fmt.Errorf("restore verification failed; symbolic link remains active: %w", err)
	}
	if err := renamePath(op.SourcePath, linkBackup, privileged); err != nil {
		_ = removeAllPath(stage, privileged)
		op.Phase, op.StagingPath, op.BackupPath = "completed", "", ""
		_ = saveOperation(op)
		return err
	}
	if err := renamePath(stage, op.SourcePath, privileged); err != nil {
		_ = renamePath(linkBackup, op.SourcePath, privileged)
		_ = removeAllPath(stage, privileged)
		op.Phase, op.StagingPath, op.BackupPath = "completed", "", ""
		_ = saveOperation(op)
		return err
	}
	op.Phase, op.StagingPath = "restoring_cleanup", ""
	if err := saveOperation(op); err != nil {
		return err
	}
	if err := removeAllPath(linkBackup, privileged); err != nil {
		return fmt.Errorf("data restored, but temporary symbolic link cleanup failed: %w", err)
	}
	op.BackupPath = ""
	_ = saveOperation(op)
	if err := os.RemoveAll(op.DestinationPath); err != nil {
		op.Error = "restored locally; external copy cleanup pending: " + err.Error()
		_ = saveOperation(op)
		return fmt.Errorf("data restored, but external copy cleanup failed: %w", err)
	}
	op.Phase, op.Error, op.BackupPath, op.StagingPath = "restored", "", "", ""
	return saveOperation(op)
}

func recoverOperations() ([]string, error) {
	operations, err := loadOperations()
	if err != nil {
		return nil, err
	}
	var results []string
	for _, op := range operations {
		if op.Phase == "completed" || op.Phase == "restored" || op.Phase == "failed" {
			continue
		}
		volumes, _ := scanVolumes()
		volume := findOperationVolume(volumes, op)
		if volume == nil {
			results = append(results, op.Name+": destination disk is offline; recovery deferred")
			continue
		}
		if op.Phase == "migrating" {
			result, err := recoverExtenderMigration(op)
			if err != nil {
				results = append(results, op.Name+": needs manual review (migration): "+err.Error())
			} else {
				results = append(results, result)
			}
			continue
		}
		localPrivileged := directoryNeedsPrivilege(filepath.Dir(op.SourcePath))
		if op.Phase == "restoring" {
			if target, err := os.Readlink(op.SourcePath); err == nil && filepath.Clean(target) == filepath.Clean(op.DestinationPath) {
				if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
					results = append(results, op.Name+": recovery needs administrator authorization")
					continue
				}
				if op.StagingPath != "" {
					_ = removeAllPath(op.StagingPath, localPrivileged)
				}
				op.Phase, op.StagingPath, op.BackupPath = "completed", "", ""
				op.Error = "interrupted before the original link was replaced"
				_ = saveOperation(op)
				results = append(results, op.Name+": removed interrupted restore copy; moved link remains active")
				continue
			}
			if _, err := os.Lstat(op.SourcePath); os.IsNotExist(err) && op.BackupPath != "" {
				if target, readErr := os.Readlink(op.BackupPath); readErr == nil && filepath.Clean(target) == filepath.Clean(op.DestinationPath) {
					if authErr := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); authErr != nil {
						results = append(results, op.Name+": recovery needs administrator authorization")
						continue
					}
					if renameErr := renamePath(op.BackupPath, op.SourcePath, localPrivileged); renameErr == nil {
						if op.StagingPath != "" {
							_ = removeAllPath(op.StagingPath, localPrivileged)
						}
						op.Phase, op.StagingPath, op.BackupPath = "completed", "", ""
						op.Error = "interrupted before restored data replaced the original link"
						_ = saveOperation(op)
						results = append(results, op.Name+": restored the original link after interruption")
						continue
					}
				}
			}
			if info, err := os.Lstat(op.SourcePath); err == nil && info.Mode()&os.ModeSymlink == 0 && verifyTree(op.SourcePath, op.DestinationPath) == nil {
				op.Phase, op.StagingPath = "restoring_cleanup", ""
				_ = saveOperation(op)
			} else {
				results = append(results, op.Name+": needs manual review (restoring)")
				continue
			}
		}
		if op.Phase == "restoring_cleanup" {
			if info, err := os.Lstat(op.SourcePath); err != nil || info.Mode()&os.ModeSymlink != 0 {
				results = append(results, op.Name+": needs manual review (restoring cleanup)")
				continue
			}
			if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
				results = append(results, op.Name+": recovery needs administrator authorization")
				continue
			}
			if op.BackupPath != "" {
				_ = removeAllPath(op.BackupPath, localPrivileged)
			}
			if err := os.RemoveAll(op.DestinationPath); err != nil {
				op.Error = "restored locally; external copy cleanup pending: " + err.Error()
				_ = saveOperation(op)
				results = append(results, op.Name+": local restore complete; external cleanup pending")
				continue
			}
			op.Phase, op.Error, op.BackupPath, op.StagingPath = "restored", "", "", ""
			_ = saveOperation(op)
			results = append(results, op.Name+": completed interrupted restore")
			continue
		}
		if op.Phase == "cleanup_pending" {
			if target, err := os.Readlink(op.SourcePath); err == nil && filepath.Clean(target) == filepath.Clean(op.DestinationPath) {
				if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
					results = append(results, op.Name+": cleanup needs administrator authorization")
					continue
				}
				if op.BackupPath != "" {
					_ = removeAllPath(op.BackupPath, localPrivileged)
				}
				op.BackupPath, op.Phase, op.Error = "", "completed", ""
				_ = saveOperation(op)
				results = append(results, op.Name+": finished cleanup")
				continue
			}
		}
		if op.Phase == "finalizing" {
			if op.StagingPath != "" {
				_ = os.RemoveAll(op.StagingPath)
			}
			if info, sourceErr := os.Lstat(op.SourcePath); sourceErr == nil && info.Mode()&os.ModeSymlink == 0 {
				_ = os.RemoveAll(op.DestinationPath)
				op.StagingPath, op.Phase = "", "failed"
				op.Error = "interrupted before source was changed; temporary copy removed"
				_ = saveOperation(op)
				results = append(results, op.Name+": removed incomplete copy; original remains")
				continue
			}
			if _, sourceErr := os.Lstat(op.SourcePath); os.IsNotExist(sourceErr) && op.BackupPath != "" && verifyTree(op.BackupPath, op.DestinationPath) == nil {
				if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
					results = append(results, op.Name+": recovery needs administrator authorization")
					continue
				}
				if err := renamePath(op.BackupPath, op.SourcePath, localPrivileged); err == nil {
					_ = os.RemoveAll(op.DestinationPath)
					op.BackupPath, op.StagingPath, op.Phase = "", "", "failed"
					op.Error = "interrupted before destination linking; original restored"
					_ = saveOperation(op)
					results = append(results, op.Name+": restored original after interruption")
					continue
				}
			}
			results = append(results, op.Name+": needs manual review (finalizing)")
			continue
		}
		if op.Phase == "copying" || op.Phase == "verifying" || op.Phase == "preparing" {
			if op.StagingPath != "" {
				_ = os.RemoveAll(op.StagingPath)
			}
			op.StagingPath, op.Phase = "", "failed"
			op.Error = "interrupted before source was changed; temporary copy removed"
			_ = saveOperation(op)
			results = append(results, op.Name+": removed incomplete copy; original remains")
			continue
		}
		if op.Phase == "linking" {
			if target, linkErr := os.Readlink(op.SourcePath); linkErr == nil && filepath.Clean(target) == filepath.Clean(op.DestinationPath) {
				verified := true
				if op.BackupPath != "" {
					if _, backupErr := os.Lstat(op.BackupPath); backupErr == nil {
						verified = verifyTree(op.BackupPath, op.DestinationPath) == nil
					}
				}
				if verified {
					if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
						results = append(results, op.Name+": cleanup needs administrator authorization")
						continue
					}
					if op.BackupPath != "" {
						_ = removeAllPath(op.BackupPath, localPrivileged)
					}
					op.BackupPath, op.Phase, op.Error = "", "completed", ""
					_ = saveOperation(op)
					results = append(results, op.Name+": completed interrupted link cleanup")
					continue
				}
			}
			if op.BackupPath != "" {
				if _, err := os.Lstat(op.SourcePath); os.IsNotExist(err) {
					if _, backupErr := os.Lstat(op.BackupPath); backupErr == nil {
						if authErr := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); authErr != nil {
							results = append(results, op.Name+": recovery needs administrator authorization")
							continue
						}
						if err := renamePath(op.BackupPath, op.SourcePath, localPrivileged); err == nil {
							_ = os.RemoveAll(op.DestinationPath)
							op.Phase, op.BackupPath = "failed", ""
							op.Error = "interrupted during link creation; original restored"
							_ = saveOperation(op)
							results = append(results, op.Name+": restored original after interruption")
							continue
						}
					}
				}
			}
			if info, statErr := os.Lstat(op.SourcePath); statErr == nil && info.Mode()&os.ModeSymlink == 0 && verifyTree(op.SourcePath, op.DestinationPath) == nil {
				_ = os.RemoveAll(op.DestinationPath)
				op.Phase, op.BackupPath, op.Error = "failed", "", "interrupted before the original path was replaced; original remains"
				_ = saveOperation(op)
				results = append(results, op.Name+": removed the verified copy; original remains in place")
				continue
			}
		}
		results = append(results, op.Name+": needs manual review ("+op.Phase+")")
	}
	return results, nil
}

func verifyTree(source, destination string) error {
	sourceManifest, err := makeManifest(source)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	destinationManifest, err := makeManifest(destination)
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	if len(sourceManifest) != len(destinationManifest) {
		return fmt.Errorf("entry count differs: source %d, destination %d", len(sourceManifest), len(destinationManifest))
	}
	for path, expected := range sourceManifest {
		actual, ok := destinationManifest[path]
		if !ok {
			return fmt.Errorf("missing item: %s", path)
		}
		if expected != actual {
			return fmt.Errorf("content differs: %s", path)
		}
	}
	return nil
}

func makeManifest(root string) (map[string]manifestEntry, error) {
	manifest := make(map[string]manifestEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			manifest[rel] = manifestEntry{Kind: "symlink", Target: target}
			if entry.IsDir() {
				return filepath.SkipDir
			}
		case info.IsDir():
			manifest[rel] = manifestEntry{Kind: "directory"}
		case info.Mode().IsRegular():
			digest, err := hashFile(path)
			if err != nil {
				return err
			}
			manifest[rel] = manifestEntry{Kind: "file", Size: info.Size(), SHA256: digest}
		default:
			return fmt.Errorf("unsupported special file at %s", path)
		}
		return nil
	})
	return manifest, err
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func requiredCapacity(bytes int64) int64 {
	if bytes < 0 {
		bytes = 0
	}
	scaled := int64(float64(bytes) * 1.1)
	reserve := bytes + 1024*1024*1024
	if reserve < bytes {
		return int64(^uint64(0) >> 1)
	}
	if scaled > reserve {
		return scaled
	}
	return reserve
}

func destinationPath(source string, volume Volume) string {
	relative := strings.TrimLeft(filepath.Clean(source), string(filepath.Separator))
	return filepath.Join(volume.MountPath, "antmove", relative)
}

func ensureSafeDestinationParent(mountPath, parentPath string) error {
	mount, err := filepath.Abs(filepath.Clean(mountPath))
	if err != nil {
		return err
	}
	parent, err := filepath.Abs(filepath.Clean(parentPath))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(mount, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("destination path escapes the selected disk")
	}
	rootInfo, err := os.Lstat(mount)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("selected disk mount path is unavailable or unsafe")
	}
	current := mount
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o700); err != nil {
				return fmt.Errorf("create destination folder %s: %w", current, err)
			}
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("destination folder is not a real directory: %s", current)
		}
	}
	return nil
}

func findVolume(volumes []Volume, uuid, mount string) *Volume {
	for i := range volumes {
		if uuid != "" && volumes[i].UUID == uuid {
			return &volumes[i]
		}
		if uuid == "" && mount != "" && filepath.Clean(volumes[i].MountPath) == filepath.Clean(mount) {
			return &volumes[i]
		}
	}
	return nil
}

func findOperationVolume(volumes []Volume, op Operation) *Volume {
	if op.VolumeUUID != "" {
		return findVolume(volumes, op.VolumeUUID, "")
	}
	return findVolume(volumes, "", op.VolumeMountPath)
}

func diskStillMounted(expected Volume) error {
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	current := findVolume(volumes, expected.UUID, expected.MountPath)
	if current == nil || filepath.Clean(current.MountPath) != filepath.Clean(expected.MountPath) {
		return fmt.Errorf("destination disk disconnected or remounted at a different path")
	}
	return nil
}

type activeActivity struct {
	Name     string
	BundleID string
	CanQuit  bool
}

func activeActivityFor(item Item) (activeActivity, bool) {
	output, err := commandOutput("/bin/ps", "-axo", "pid=,command=")
	if err != nil {
		return activeActivity{}, false
	}
	identity := appIdentityFor(item)
	if strings.EqualFold(filepath.Ext(item.SourcePath), ".app") {
		identity = readAppIdentity(item.SourcePath)
	}
	needles := []string{}
	switch item.Category {
	case CategoryDeveloper:
		needles = append(needles, "xcodebuild", "Simulator.app", "Android Studio", "sdkmanager", "emulator")
	case CategoryContainer:
		needles = append(needles, "com.docker.backend", "OrbStack", "colima", "lima", "rancher-desktop")
	case CategoryAppleBackup:
		// These background agents can be idle while no backup is running.
		needles = append(needles, "MobileBackup")
	}
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		separator := strings.IndexAny(trimmed, " \t")
		if separator < 0 || trimmed[:separator] == fmt.Sprint(os.Getpid()) {
			continue
		}
		processCommand := strings.TrimSpace(trimmed[separator:])
		if processCommand == "" {
			continue
		}
		processExecutable := processExecutablePath(processCommand)
		lowerExecutable := strings.ToLower(processExecutable)
		appExecutablePath := ""
		if identity.Path != "" {
			appExecutablePath = strings.ToLower(filepath.Join(identity.Path, "Contents", "MacOS")) + string(os.PathSeparator)
		}
		if appExecutablePath != "" && strings.HasPrefix(lowerExecutable, appExecutablePath) && identity.BundleID != "" {
			name := identity.Name
			if name == "" {
				name = item.Name
			}
			return activeActivity{Name: name, BundleID: identity.BundleID, CanQuit: true}, true
		}
		binary := filepath.Base(processExecutable)
		if binary == "zsh" || binary == "bash" || binary == "sh" {
			continue
		}
		if identity.BundleID != "" && identity.Executable != "" && identity.Path == "" && strings.EqualFold(binary, identity.Executable) {
			return activeActivity{Name: identity.Name, BundleID: identity.BundleID, CanQuit: true}, true
		}
		for _, needle := range needles {
			if needle != "" && strings.Contains(lowerExecutable, strings.ToLower(needle)) {
				return activeActivity{Name: needle}, true
			}
		}
	}
	return activeActivity{}, false
}

func processExecutablePath(processCommand string) string {
	lower := strings.ToLower(processCommand)
	marker := ".app/contents/macos/"
	if index := strings.Index(lower, marker); index >= 0 {
		end := index + len(marker)
		if space := strings.IndexAny(processCommand[end:], " \t"); space >= 0 {
			return processCommand[:end+space]
		}
		return processCommand
	}
	if space := strings.IndexAny(processCommand, " \t"); space >= 0 {
		return processCommand[:space]
	}
	return processCommand
}

func activeProcessFor(item Item) error {
	if activity, ok := activeActivityFor(item); ok {
		return fmt.Errorf("related app or process %s is active; close it safely before continuing", activity.Name)
	}
	return nil
}

func activityError(item Item, activity activeActivity) error {
	if activity.CanQuit {
		return fmt.Errorf("%s is running. Review the warning and use the interactive close-app confirmation or --close-apps to let antmove request permission to quit it", activity.Name)
	}
	return fmt.Errorf("related process %s is active. Finish its work and rescan before moving %s; antmove will not terminate background services", activity.Name, item.Name)
}

func appIdentityFor(item Item) appIdentity {
	if strings.EqualFold(filepath.Ext(item.SourcePath), ".app") {
		return readAppIdentity(item.SourcePath)
	}
	owner := strings.ToLower(item.Owner)
	if owner == "" {
		switch normalizeAppName(item.Name) {
		case "applemusicmedia":
			owner = "music"
		case "appletvmedia", "appletvdownloads":
			owner = "tv"
		case "photoslibrarydefault":
			owner = "photos"
		}
		if strings.EqualFold(filepath.Ext(item.SourcePath), ".photoslibrary") {
			owner = "photos"
		}
	}
	switch owner {
	case "music":
		return appIdentity{Name: "Music", BundleID: "com.apple.Music", Executable: "Music", Path: "/System/Applications/Music.app"}
	case "tv":
		return appIdentity{Name: "TV", BundleID: "com.apple.TV", Executable: "TV", Path: "/System/Applications/TV.app"}
	case "photos":
		return appIdentity{Name: "Photos", BundleID: "com.apple.Photos", Executable: "Photos", Path: "/System/Applications/Photos.app"}
	}
	if item.Owner == "" {
		return appIdentity{}
	}
	home, _ := os.UserHomeDir()
	for _, root := range []string{"/Applications", "/System/Applications", filepath.Join(home, "Applications")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
				continue
			}
			appPath := filepath.Join(root, entry.Name())
			identity := readAppIdentity(appPath)
			if normalizeAppName(identity.Name) == normalizeAppName(item.Owner) || normalizeAppName(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))) == normalizeAppName(item.Owner) {
				return identity
			}
		}
	}
	return appIdentity{}
}

func quitApplication(activity activeActivity) error {
	if !activity.CanQuit || activity.BundleID == "" {
		return fmt.Errorf("%s cannot be quit safely by antmove", activity.Name)
	}
	script := fmt.Sprintf("tell application id %q to quit", activity.BundleID)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("timed out waiting for %s to quit; it may need attention to save work. no files were moved", activity.Name)
		}
		return fmt.Errorf("could not get a graceful quit from %s; it may be waiting for unsaved-work confirmation, or macOS may have denied Automation permission. no files were moved. if denied, allow the requesting app under System Settings > Privacy & Security > Automation, then retry: %s", activity.Name, strings.TrimSpace(string(output)))
	}
	return nil
}

func closeAppsForItems(items []Item) error {
	type target struct {
		item     Item
		activity activeActivity
	}
	targets := make([]target, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		activity, active := activeActivityFor(item)
		if !active {
			continue
		}
		if !activity.CanQuit {
			return activityError(item, activity)
		}
		if !seen[activity.BundleID] {
			seen[activity.BundleID] = true
			targets = append(targets, target{item: item, activity: activity})
		}
	}
	for _, target := range targets {
		if err := quitApplication(target.activity); err != nil {
			return err
		}
	}
	for _, target := range targets {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, active := activeActivityFor(target.item); !active {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		if activity, active := activeActivityFor(target.item); active {
			return fmt.Errorf("%s is still active; no files were moved", activity.Name)
		}
	}
	return nil
}

func randomID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
