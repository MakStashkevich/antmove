package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
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
	return moveItemWithProgress(item, volume, dryRun, nil)
}

func moveItemWithProgress(item Item, volume Volume, dryRun bool, progress func(int64)) (Operation, error) {
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
	output, skippedSockets, copyErr := copyTreeIgnoringInactiveSocketsProgress(item.SourcePath, staging, progress)
	if copyErr != nil {
		op.Phase, op.Error = "failed", strings.TrimSpace(string(output))
		if op.Error == "" {
			op.Error = copyErr.Error()
		}
		_ = saveOperation(op)
		_ = os.RemoveAll(staging)
		return op, fmt.Errorf("copy failed: %w: %s", copyErr, strings.TrimSpace(string(output)))
	}
	if skippedSockets > 0 {
		op.Warning = fmt.Sprintf("Omitted %d inactive Unix socket runtime endpoint(s); they cannot be relocated.", skippedSockets)
	}
	op.Phase = "verifying"
	if err := saveOperation(op); err != nil {
		return op, err
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
	if err := removeVerifiedBackup(op, backup, finalPath, privileged); err != nil {
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
					if err := removeVerifiedBackup(op, op.BackupPath, op.DestinationPath, localPrivileged); err != nil {
						op.Error = "linked successfully; backup cleanup pending: " + err.Error()
						_ = saveOperation(op)
						results = append(results, op.Name+": backup preserved; cleanup still pending: "+err.Error())
						continue
					}
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

func removeVerifiedBackup(op Operation, backup, destination string, privileged bool) error {
	if filepath.Clean(filepath.Dir(backup)) != filepath.Clean(filepath.Dir(op.SourcePath)) || !strings.HasPrefix(filepath.Base(backup), ".antmove-backup-") {
		return fmt.Errorf("backup path is outside the expected Antmove backup location")
	}
	empty, err := cleanFinderMetadataEntries(backup)
	if err != nil {
		return fmt.Errorf("remove Finder metadata from backup: %w", err)
	}
	if empty {
		if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if op.Category == CategoryAIData {
		activityItem := Item{Name: op.Name, Category: op.Category, SourcePath: op.SourcePath, Owner: op.Name}
		if activity, active := activeActivityFor(activityItem); active {
			return fmt.Errorf("%s is still active; close it before retrying backup cleanup", activity.Name)
		}
	}
	if err := verifyBackupContents(backup, destination, op.Category == CategoryAIData); err != nil {
		return fmt.Errorf("backup differs from the verified destination and was preserved: %w", err)
	}
	return removeAllPath(backup, privileged)
}

func isFinderMetadataName(name string) bool {
	return name == ".DS_Store" || strings.HasPrefix(name, "._")
}

// cleanFinderMetadataEntries removes only Finder's directory metadata and
// AppleDouble sidecars, then prunes directories that became empty.
func cleanFinderMetadataEntries(root string) (bool, error) {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("backup path is not an ordinary directory")
	}
	var directories []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root {
				directories = append(directories, path)
			}
			return nil
		}
		if !isFinderMetadataName(entry.Name()) {
			return nil
		}
		fileInfo, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			return nil
		}
		return os.Remove(path)
	})
	if err != nil {
		return false, err
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		if err := os.Remove(directory); err != nil && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func verifyBackupContents(backup, destination string, aiData bool) error {
	if aiData {
		for _, root := range []string{backup, destination} {
			sockets, _, err := findRuntimeSockets(root)
			if err != nil {
				return err
			}
			for _, socket := range sockets {
				owners, err := unixSocketOwners(socket.path)
				if err != nil {
					return err
				}
				if owners != "" {
					return fmt.Errorf("runtime socket %s is still open by PID(s) %s", socket.path, owners)
				}
			}
		}
	}
	return verifyBackupSubset(backup, destination, aiData)
}

func verifyBackupSubset(backup, destination string, omitSockets bool) error {
	manifest := makeManifest
	if omitSockets {
		manifest = makeManifestOmittingSockets
	}
	backupManifest, err := manifest(backup)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	destinationManifest, err := manifest(destination)
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	for path := range backupManifest {
		if isFinderMetadataName(filepath.Base(filepath.FromSlash(path))) {
			delete(backupManifest, path)
		}
	}
	for path := range destinationManifest {
		if isFinderMetadataName(filepath.Base(filepath.FromSlash(path))) {
			delete(destinationManifest, path)
		}
	}
	for path, expected := range backupManifest {
		if actual, ok := destinationManifest[path]; !ok {
			return fmt.Errorf("destination is missing backup item: %s", path)
		} else if expected != actual {
			return fmt.Errorf("content differs: %s", path)
		}
	}
	return nil
}

func makeManifestOmittingSockets(root string) (map[string]manifestEntry, error) {
	manifest := make(map[string]manifestEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSocket != 0 {
			return nil
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

type runtimeSocketEntry struct {
	path   string
	parked string
}

type directoryTimes struct {
	path  string
	atime time.Time
	mtime time.Time
}

func copyTreeIgnoringInactiveSockets(source, destination string) ([]byte, int, error) {
	return copyTreeIgnoringInactiveSocketsProgress(source, destination, nil)
}

func copyTreeIgnoringInactiveSocketsProgress(source, destination string, progress func(int64)) ([]byte, int, error) {
	sockets, parents, err := findRuntimeSockets(source)
	if err != nil {
		return nil, 0, fmt.Errorf("inspect runtime sockets: %w", err)
	}
	if len(sockets) == 0 {
		output, err := dittoCopyProgress(source, destination, false, progress)
		if err != nil {
			return output, 0, err
		}
		if err := verifyTree(source, destination); err != nil {
			return output, 0, fmt.Errorf("verification failed: %w", err)
		}
		return output, 0, nil
	}
	for _, socket := range sockets {
		owners, err := unixSocketOwners(socket.path)
		if err != nil {
			return nil, len(sockets), fmt.Errorf("could not verify whether Unix socket %s is active: %w", socket.path, err)
		}
		if owners != "" {
			return nil, len(sockets), fmt.Errorf("Unix socket %s is still open by process PID(s) %s; stop that assistant service and rescan", socket.path, owners)
		}
	}
	quarantine, err := os.MkdirTemp(filepath.Dir(source), ".antmove-runtime-sockets-")
	if err != nil {
		return nil, len(sockets), fmt.Errorf("create temporary socket quarantine beside source: %w", err)
	}
	for i := range sockets {
		sockets[i].parked = filepath.Join(quarantine, strconv.Itoa(i))
		if err := os.Rename(sockets[i].path, sockets[i].parked); err != nil {
			restoreErr := restoreRuntimeSockets(sockets, parents, quarantine)
			if restoreErr != nil {
				return nil, len(sockets), fmt.Errorf("temporarily move runtime socket %s: %w; restore failed: %v", sockets[i].path, err, restoreErr)
			}
			return nil, len(sockets), fmt.Errorf("temporarily move inactive runtime socket %s: %w", sockets[i].path, err)
		}
	}
	for _, parent := range parents {
		if err := os.Chtimes(parent.path, parent.atime, parent.mtime); err != nil {
			restoreErr := restoreRuntimeSockets(sockets, parents, quarantine)
			if restoreErr != nil {
				return nil, len(sockets), fmt.Errorf("restore directory metadata for %s: %w; socket restore failed: %v", parent.path, err, restoreErr)
			}
			return nil, len(sockets), fmt.Errorf("restore directory metadata while excluding inactive socket: %w", err)
		}
	}
	output, copyErr := dittoCopyProgress(source, destination, false, progress)
	if copyErr == nil {
		copyErr = verifyTree(source, destination)
		if copyErr != nil {
			copyErr = fmt.Errorf("verification failed: %w", copyErr)
		}
	}
	if err := restoreRuntimeSockets(sockets, parents, quarantine); err != nil {
		return output, len(sockets), fmt.Errorf("could not restore inactive source sockets after copy: %w", err)
	}
	if copyErr != nil {
		return output, len(sockets), copyErr
	}
	return output, len(sockets), nil
}

func findRuntimeSockets(root string) ([]runtimeSocketEntry, []directoryTimes, error) {
	var sockets []runtimeSocketEntry
	parents := map[string]directoryTimes{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSocket == 0 {
			return nil
		}
		parentPath := filepath.Dir(path)
		if _, exists := parents[parentPath]; !exists {
			parentInfo, err := os.Stat(parentPath)
			if err != nil {
				return err
			}
			atime := parentInfo.ModTime()
			if raw, ok := parentInfo.Sys().(*syscall.Stat_t); ok {
				atime = time.Unix(raw.Atimespec.Sec, raw.Atimespec.Nsec)
			}
			parents[parentPath] = directoryTimes{path: parentPath, atime: atime, mtime: parentInfo.ModTime()}
		}
		sockets = append(sockets, runtimeSocketEntry{path: path})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	parentList := make([]directoryTimes, 0, len(parents))
	for _, parent := range parents {
		parentList = append(parentList, parent)
	}
	sort.Slice(parentList, func(i, j int) bool { return parentList[i].path < parentList[j].path })
	return sockets, parentList, nil
}

func unixSocketOwners(path string) (string, error) {
	command := exec.Command("/usr/sbin/lsof", "-nP", "-t", path)
	output, err := command.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && strings.TrimSpace(string(output)) == "" {
		return "", nil
	}
	return "", fmt.Errorf("lsof failed: %w: %s", err, strings.TrimSpace(string(output)))
}

func restoreRuntimeSockets(sockets []runtimeSocketEntry, parents []directoryTimes, quarantine string) error {
	var failures []string
	for i := len(sockets) - 1; i >= 0; i-- {
		socket := sockets[i]
		if socket.parked == "" {
			continue
		}
		if _, err := os.Lstat(socket.parked); os.IsNotExist(err) {
			continue
		}
		if err := os.Rename(socket.parked, socket.path); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", socket.path, err))
		}
	}
	for _, parent := range parents {
		if err := os.Chtimes(parent.path, parent.atime, parent.mtime); err != nil {
			failures = append(failures, fmt.Sprintf("restore directory time %s: %v", parent.path, err))
		}
	}
	if len(failures) == 0 {
		if err := os.RemoveAll(quarantine); err != nil {
			failures = append(failures, fmt.Sprintf("remove temporary quarantine %s: %v", quarantine, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
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
	Name         string
	BundleID     string
	HostPath     string
	CanQuit      bool
	ForceHelpers bool
	PID          int
	Process      string
}

type processInfo struct {
	pid     int
	ppid    int
	command string
}

func activeActivityFor(item Item) (activeActivity, bool) {
	output, err := commandOutput("/bin/ps", "-axo", "pid=,ppid=,command=")
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
	case CategoryAIData:
		needles = aiAssistantNamesForItem(item)
	case CategoryContainer:
		needles = append(needles, "com.docker.backend", "OrbStack", "colima", "lima", "rancher-desktop")
	case CategoryAppleBackup:
		// These background agents can be idle while no backup is running.
		needles = append(needles, "MobileBackup")
	}
	processes := parseProcessTable(string(output))
	byPID := make(map[int]processInfo, len(processes))
	for _, process := range processes {
		byPID[process.pid] = process
	}
	for _, process := range processes {
		if process.pid == os.Getpid() {
			continue
		}
		processCommand := process.command
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
			forceHelpers := item.Category == CategoryAIData && aiAssistantProcessMatches(identity.Name, aiAssistantNamesForItem(item))
			return activeActivity{Name: name, BundleID: identity.BundleID, HostPath: identity.Path, CanQuit: true, ForceHelpers: forceHelpers, PID: process.pid, Process: filepath.Base(processExecutable)}, true
		}
		binary := filepath.Base(processExecutable)
		if binary == "zsh" || binary == "bash" || binary == "sh" {
			continue
		}
		if identity.BundleID != "" && identity.Executable != "" && identity.Path == "" && strings.EqualFold(binary, identity.Executable) {
			forceHelpers := item.Category == CategoryAIData && aiAssistantProcessMatches(identity.Name, aiAssistantNamesForItem(item))
			return activeActivity{Name: identity.Name, BundleID: identity.BundleID, HostPath: identity.Path, CanQuit: true, ForceHelpers: forceHelpers, PID: process.pid, Process: binary}, true
		}
		if item.Category == CategoryAIData {
			app := appIdentity{}
			if appPath := enclosingAppBundleForProcess(process, byPID); appPath != "" {
				app = readAppIdentity(appPath)
			}
			if activity, active := aiAssistantActivityForProcess(item, binary, app); active {
				activity.PID, activity.Process, activity.HostPath, activity.ForceHelpers = process.pid, binary, app.Path, true
				return activity, true
			}
			if processUsesDataFolder(processExecutable, item.SourcePath) {
				if app.BundleID != "" && isKnownAIHostApp(app.Name) {
					return activeActivity{Name: app.Name, BundleID: app.BundleID, HostPath: app.Path, CanQuit: true, PID: process.pid, Process: binary}, true
				}
				return activeActivity{Name: binary, PID: process.pid, Process: binary}, true
			}
			continue
		}
		for _, needle := range needles {
			if needle != "" && strings.Contains(lowerExecutable, strings.ToLower(needle)) {
				return activeActivity{Name: needle}, true
			}
		}
	}
	return activeActivity{}, false
}

func parseProcessTable(output string) []processInfo {
	var processes []processInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		firstSeparator := strings.IndexAny(line, " \t")
		if firstSeparator <= 0 {
			continue
		}
		rest := strings.TrimSpace(line[firstSeparator:])
		secondSeparator := strings.IndexAny(rest, " \t")
		if secondSeparator <= 0 {
			continue
		}
		pid, pidErr := strconv.Atoi(strings.TrimSpace(line[:firstSeparator]))
		ppid, ppidErr := strconv.Atoi(strings.TrimSpace(rest[:secondSeparator]))
		command := strings.TrimSpace(rest[secondSeparator:])
		if pidErr != nil || ppidErr != nil || command == "" {
			continue
		}
		processes = append(processes, processInfo{pid: pid, ppid: ppid, command: command})
	}
	return processes
}

func enclosingAppBundleForProcess(process processInfo, byPID map[int]processInfo) string {
	seen := make(map[int]bool)
	for depth := 0; depth < 16 && process.pid > 0 && !seen[process.pid]; depth++ {
		seen[process.pid] = true
		if appPath := enclosingAppBundle(processExecutablePath(process.command)); appPath != "" {
			return appPath
		}
		parent, ok := byPID[process.ppid]
		if !ok || parent.pid <= 1 {
			break
		}
		process = parent
	}
	return ""
}

func isKnownAIHostApp(name string) bool {
	switch normalizeAppName(name) {
	case "zed", "visualstudiocode", "code", "vscodium", "cursor", "windsurf", "antigravity",
		"claude", "chatgpt", "opencode", "gemini", "kiro", "trae", "replit", "warp",
		"fleet", "intellijidea", "pycharm", "webstorm", "goland", "rider", "androidstudio":
		return true
	default:
		return false
	}
}

func aiAssistantActivityForProcess(item Item, binary string, app appIdentity) (activeActivity, bool) {
	if item.Category != CategoryAIData {
		return activeActivity{}, false
	}
	names := aiAssistantNamesForItem(item)
	if !aiAssistantProcessMatches(binary, names) && !aiAssistantProcessMatches(app.Name, names) {
		return activeActivity{}, false
	}
	if app.BundleID != "" && (aiAssistantProcessMatches(app.Name, names) || isKnownAIHostApp(app.Name)) {
		name := app.Name
		if name == "" {
			name = binary
		}
		return activeActivity{Name: name, BundleID: app.BundleID, HostPath: app.Path, CanQuit: true}, true
	}
	return activeActivity{Name: binary}, true
}

func aiAssistantNamesForItem(item Item) []string {
	name := normalizeAppName(filepath.Base(item.SourcePath))
	all := []string{
		"codex", "chatgpt", "claude", "cursor", "copilot", "kilocode", "kilo", "hermes", "roo", "cline", "continue", "aider",
		"windsurf", "codeium", "gemini", "antigravity", "qwen", "opencode", "goose", "amp", "zed", "kiro", "amazonq",
		"junie", "openclaw", "pi", "crush", "factory", "droid", "kimi", "trae", "warp", "replit", "void", "cody",
		"tabby", "ollama", "lmstudio", "jan", "anythingllm", "msty", "gpt4all", "perplexity",
	}
	aliases := map[string][]string{
		"codex": {"codex", "chatgpt"}, "chatgpt": {"chatgpt", "codex"},
		"gemini": {"gemini", "antigravity"}, "antigravity": {"antigravity", "gemini"},
		"geminicli": {"gemini", "antigravity"}, "antigravityide": {"antigravity", "gemini"}, "antigravitycli": {"antigravity", "gemini"},
		"claudecode": {"claude", "claudecode"}, "copilot": {"copilot", "githubcopilot"}, "githubcopilot": {"copilot", "githubcopilot"},
		"copilotcli": {"copilot", "githubcopilot"}, "qwencode": {"qwen", "qwencode"}, "qwencodecli": {"qwen", "qwencode"},
		"opencode": {"opencode"}, "sourcegraphcody": {"cody", "sourcegraphcody"}, "roocode": {"roo", "roocode"},
		"kilocode": {"kilocode", "kilo"}, "kilo": {"kilo", "kilocode"},
		"amazonq": {"amazonq", "kiro"}, "kiro": {"kiro", "amazonq"},
		"factory": {"factory", "droid"}, "droid": {"droid", "factory"},
		"agents": all,
	}
	if names, ok := aliases[name]; ok {
		return names
	}
	if name == "" {
		return nil
	}
	return []string{name}
}

func aiAssistantProcessMatches(binary string, names []string) bool {
	processName := normalizeAppName(binary)
	if processName == "" {
		return false
	}
	for _, name := range names {
		if processName == normalizeAppName(name) {
			return true
		}
	}
	return false
}

func enclosingAppBundle(path string) string {
	path = filepath.Clean(path)
	current := string(os.PathSeparator)
	for _, part := range strings.Split(strings.TrimPrefix(path, current), current) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		if strings.EqualFold(filepath.Ext(part), ".app") {
			return current
		}
	}
	return ""
}

func processUsesDataFolder(processPath, dataPath string) bool {
	relative, err := filepath.Rel(filepath.Clean(dataPath), filepath.Clean(processPath))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative)
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

func refreshItemAfterAppQuit(item Item) (Item, error) {
	info, err := os.Lstat(item.SourcePath)
	if err != nil {
		return Item{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return Item{}, fmt.Errorf("source is no longer an ordinary file or directory")
	}
	size, sockets, err := allocatedSizeWithRuntimeSockets(item.SourcePath)
	if err != nil {
		return Item{}, err
	}
	item.SourceFingerprint, item.SizeBytes, item.RuntimeSockets = fingerprint(info), size, sockets
	return item, nil
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
	byBundle := map[string]int{}
	for _, item := range items {
		activity, active := activeActivityFor(item)
		if !active {
			continue
		}
		if !activity.CanQuit {
			return activityError(item, activity)
		}
		if index, seen := byBundle[activity.BundleID]; seen {
			targets[index].activity.ForceHelpers = targets[index].activity.ForceHelpers || activity.ForceHelpers
			if targets[index].activity.HostPath == "" {
				targets[index].activity.HostPath = activity.HostPath
			}
		} else {
			byBundle[activity.BundleID] = len(targets)
			targets = append(targets, target{item: item, activity: activity})
		}
	}
	for _, target := range targets {
		if err := quitApplication(target.activity); err != nil {
			return err
		}
	}
	for _, target := range targets {
		if target.activity.ForceHelpers && target.activity.HostPath != "" {
			if err := waitForAIHostQuit(target.activity, 10*time.Second); err != nil {
				return err
			}
			if err := terminateAIHostHelpers(target.item, target.activity); err != nil {
				return err
			}
		} else {
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				if _, active := activeActivityFor(target.item); !active {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
		}
		if activity, active := activeActivityFor(target.item); active {
			if activity.PID > 0 && activity.Process != "" {
				return fmt.Errorf("%s is still active (process %s, PID %d); no files were moved. Quit that process through its app or terminal, then rescan", activity.Name, activity.Process, activity.PID)
			}
			return fmt.Errorf("%s is still active; no files were moved", activity.Name)
		}
	}
	return nil
}

func waitForAIHostQuit(activity activeActivity, timeout time.Duration) error {
	mainExecutable := filepath.Join(activity.HostPath, "Contents", "MacOS")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		processes, err := currentProcessTable()
		if err != nil {
			return fmt.Errorf("could not verify that %s closed; no files were moved: %w", activity.Name, err)
		}
		if !hasMainAppProcess(processes, activity.HostPath, mainExecutable) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("%s did not close after the graceful quit request; helper processes were left untouched and no files were moved", activity.Name)
}

func currentProcessTable() ([]processInfo, error) {
	output, err := commandOutput("/bin/ps", "-axo", "pid=,ppid=,command=")
	if err != nil {
		return nil, err
	}
	return parseProcessTable(string(output)), nil
}

func hasMainAppProcess(processes []processInfo, hostPath, mainDirectory string) bool {
	for _, process := range processes {
		if process.pid == os.Getpid() {
			continue
		}
		executable := filepath.Clean(processExecutablePath(process.command))
		if strings.HasPrefix(executable, filepath.Clean(mainDirectory)+string(os.PathSeparator)) && enclosingAppBundle(executable) == filepath.Clean(hostPath) {
			return true
		}
	}
	return false
}

func terminateAIHostHelpers(item Item, activity activeActivity) error {
	if item.Category != CategoryAIData || activity.HostPath == "" {
		return nil
	}
	processes, err := currentProcessTable()
	if err != nil {
		return fmt.Errorf("could not inspect %s helper processes; no files were moved: %w", activity.Name, err)
	}
	mainDirectory := filepath.Join(activity.HostPath, "Contents", "MacOS")
	if hasMainAppProcess(processes, activity.HostPath, mainDirectory) {
		return fmt.Errorf("%s is still running; Antmove will not force-quit the main app. No files were moved", activity.Name)
	}
	type helperProcess struct {
		pid        int
		executable string
	}
	var helpers []helperProcess
	for _, process := range processes {
		if process.pid == os.Getpid() {
			continue
		}
		executable := filepath.Clean(processExecutablePath(process.command))
		if isAIHostHelperExecutable(executable, activity.HostPath, mainDirectory) {
			helpers = append(helpers, helperProcess{pid: process.pid, executable: executable})
		}
	}
	for _, helper := range helpers {
		// Recheck PID identity immediately before signaling to avoid hitting a reused PID.
		current, err := currentProcessTable()
		if err != nil {
			return fmt.Errorf("could not recheck %s helper PID %d; no files were moved: %w", activity.Name, helper.pid, err)
		}
		stillSame := false
		for _, process := range current {
			if process.pid == helper.pid && filepath.Clean(processExecutablePath(process.command)) == helper.executable &&
				isAIHostHelperExecutable(helper.executable, activity.HostPath, mainDirectory) {
				stillSame = true
				break
			}
		}
		if !stillSame {
			continue
		}
		process, err := os.FindProcess(helper.pid)
		if err != nil {
			return fmt.Errorf("could not find %s helper PID %d; no files were moved: %w", activity.Name, helper.pid, err)
		}
		if err := process.Kill(); err != nil {
			return fmt.Errorf("could not force-quit %s helper PID %d; no files were moved: %w", activity.Name, helper.pid, err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		active, ok := activeActivityFor(item)
		if !ok {
			return nil
		}
		if active.HostPath != "" && filepath.Clean(active.HostPath) == filepath.Clean(activity.HostPath) {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		return activityError(item, active)
	}
	if active, ok := activeActivityFor(item); ok {
		return fmt.Errorf("%s helper process %s (PID %d) remains active; no files were moved", activity.Name, active.Process, active.PID)
	}
	return nil
}

func isAIHostHelperExecutable(executable, hostPath, mainDirectory string) bool {
	executable = filepath.Clean(executable)
	prefix := filepath.Join(hostPath, "Contents") + string(os.PathSeparator)
	mainPrefix := filepath.Clean(mainDirectory) + string(os.PathSeparator)
	return strings.HasPrefix(executable, prefix) && !strings.HasPrefix(executable, mainPrefix)
}

func randomID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
