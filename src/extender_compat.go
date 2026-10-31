package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Extender stored each transfer at <volume>/Extender/<original absolute path>
// and replaced the original path with a symbolic link. The stable layout lets
// antmove safely recognize completed transfers even though Extender removed its
// transfer journal after success.
func extenderTransferMount(source, destination string) (string, bool) {
	source = filepath.Clean(source)
	destination = filepath.Clean(destination)
	if !filepath.IsAbs(source) || source == string(filepath.Separator) || !filepath.IsAbs(destination) {
		return "", false
	}
	relativeDestination, err := filepath.Rel("/Volumes", destination)
	if err != nil || relativeDestination == "." || relativeDestination == ".." || strings.HasPrefix(relativeDestination, ".."+string(filepath.Separator)) {
		return "", false
	}
	parts := strings.Split(relativeDestination, string(filepath.Separator))
	if len(parts) < 3 || parts[0] == "" || parts[1] != "Extender" {
		return "", false
	}
	mount := filepath.Join("/Volumes", parts[0])
	expected := filepath.Join(mount, "Extender", strings.TrimLeft(source, string(filepath.Separator)))
	if filepath.Clean(expected) != destination {
		return "", false
	}
	return mount, true
}

func extenderLinkDestination(source string) (string, bool) {
	info, err := os.Lstat(source)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return "", false
	}
	raw, err := os.Readlink(source)
	if err != nil {
		return "", false
	}
	destination := raw
	if !filepath.IsAbs(destination) {
		destination = filepath.Join(filepath.Dir(source), destination)
	}
	destination = filepath.Clean(destination)
	if _, ok := extenderTransferMount(source, destination); !ok {
		return "", false
	}
	return destination, true
}

func isExtenderManagedLink(source string) bool {
	_, ok := extenderLinkDestination(source)
	return ok
}

func importExtenderTransfers(items map[string]Item, existing []Operation) ([]Operation, error) {
	bySource := make(map[string]bool, len(existing))
	byID := make(map[string]bool, len(existing))
	for _, op := range existing {
		bySource[filepath.Clean(op.SourcePath)] = true
		byID[op.ID] = true
	}
	volumes, _ := scanVolumes()

	paths := make([]string, 0, len(items))
	for path := range items {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := append([]Operation(nil), existing...)
	for _, path := range paths {
		item := items[path]
		if bySource[filepath.Clean(path)] {
			continue
		}
		destination, ok := extenderLinkDestination(path)
		if !ok || (item.DestinationPath != "" && filepath.Clean(item.DestinationPath) != destination) {
			continue
		}
		mount, _ := extenderTransferMount(path, destination)
		volume := findVolume(volumes, "", mount)
		volumeName := filepath.Base(mount)
		volumeUUID := ""
		if volume != nil {
			volumeName, volumeUUID = volume.Name, volume.UUID
		}
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		size := item.SizeBytes
		if measured, err := allocatedSize(destination); err == nil {
			size = measured
		}
		name := extenderOperationName(path, item.Name)
		id := "extender-" + shortID(path)
		if byID[id] {
			return nil, fmt.Errorf("cannot import Extender transfer at %s: operation ID collision", path)
		}
		op := Operation{
			ID: id, ItemID: item.ID, Origin: "Extender", Name: name, Category: item.Category,
			SourcePath: path, DestinationPath: destination, VolumeUUID: volumeUUID,
			VolumeMountPath: mount, VolumeName: volumeName, SizeBytes: size,
			Phase: "completed", CreatedAt: info.ModTime(), UpdatedAt: time.Now(),
		}
		if err := saveOperation(op); err != nil {
			return nil, fmt.Errorf("import Extender transfer %s: %w", item.Name, err)
		}
		bySource[filepath.Clean(path)], byID[id] = true, true
		result = append(result, op)
		item.Status, item.DestinationPath, item.VolumeUUID = StatusMoved, destination, volumeUUID
		item.Reason = "Previously moved by Extender; available to restore."
		items[path] = item
	}
	for i := range result {
		op := result[i]
		if op.Origin != "Extender" {
			continue
		}
		if name := extenderOperationName(op.SourcePath, op.Name); name != op.Name {
			op.Name = name
			if err := saveOperation(op); err != nil {
				return nil, fmt.Errorf("update imported Extender transfer %s: %w", op.SourcePath, err)
			}
			result[i] = op
		}
		item, ok := items[op.SourcePath]
		if !ok {
			continue
		}
		item.Name = op.Name
		item.Reason = "Previously moved by Extender; available to restore."
		if item.VolumeUUID == "" {
			item.VolumeUUID = op.VolumeUUID
		}
		items[op.SourcePath] = item
	}
	return result, nil
}

func extenderOperationName(source, fallback string) string {
	home, _ := os.UserHomeDir()
	switch filepath.Clean(source) {
	case filepath.Join(home, "Library", "Android", "sdk"):
		return "Android SDK"
	case filepath.Join(home, ".android", "avd"):
		return "Android Virtual Devices"
	default:
		return fallback
	}
}

func runMigrate(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dryRun := flags.Bool("dry-run", false, "preview Extender data migration without changing files")
	yes := flags.Bool("yes", false, "confirm migration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := scanItems(); err != nil {
		return err
	}
	operations, err := loadOperations()
	if err != nil {
		return err
	}
	var pending []Operation
	for _, op := range operations {
		if op.Origin != "Extender" || op.Phase != "completed" || op.PreviousDestinationPath != "" {
			continue
		}
		destination, ok := extenderLinkDestination(op.SourcePath)
		if ok && filepath.Clean(destination) == filepath.Clean(op.DestinationPath) {
			pending = append(pending, op)
		}
	}
	if len(pending) == 0 {
		fmt.Println("No Extender transfers need migration.")
		return nil
	}
	for _, op := range pending {
		fmt.Printf("%s %s: %s → %s\n", map[bool]string{true: "Would move", false: "Moving"}[*dryRun],
			op.Name, op.DestinationPath, destinationForImportedOperation(op))
	}
	if *dryRun {
		return nil
	}
	if !*yes {
		if !isTerminal() {
			return errors.New("non-interactive migration requires --yes")
		}
		if !confirm(fmt.Sprintf("Move %d Extender transfer(s) to the antmove folder? [y/N] ", len(pending))) {
			return errors.New("cancelled")
		}
	}
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	var failures []string
	for _, op := range pending {
		fmt.Printf("Verifying SHA-256 before moving %s; large folders may take a while…\n", op.Name)
		volume := findOperationVolume(volumes, op)
		if volume == nil {
			message := fmt.Sprintf("%s: destination disk %q is not connected", op.Name, op.VolumeName)
			fmt.Println("Skipped " + message)
			failures = append(failures, message)
			continue
		}
		if err := checkMigrationVolume(*volume); err != nil {
			message := fmt.Sprintf("%s: %v", op.Name, err)
			fmt.Println("Skipped " + message)
			failures = append(failures, message)
			continue
		}
		if err := migrateExtenderOperation(op, *volume); err != nil {
			message := fmt.Sprintf("%s: %v", op.Name, err)
			fmt.Println("Skipped " + message)
			failures = append(failures, message)
			continue
		}
		fmt.Printf("✓ Migrated %s to %s\n", op.Name, destinationPath(op.SourcePath, *volume))
	}
	if len(failures) != 0 {
		return fmt.Errorf("%d transfer(s) could not be migrated; resolve the notes above and run am migrate again", len(failures))
	}
	return nil
}

func destinationForImportedOperation(op Operation) string {
	mount := op.VolumeMountPath
	if mount == "" {
		mount, _ = extenderTransferMount(op.SourcePath, op.DestinationPath)
	}
	return destinationPath(op.SourcePath, Volume{MountPath: mount})
}

func migrateExtenderOperation(op Operation, volume Volume) error {
	oldDestination, ok := extenderLinkDestination(op.SourcePath)
	if !ok || filepath.Clean(oldDestination) != filepath.Clean(op.DestinationPath) {
		return fmt.Errorf("source is no longer a verified Extender link")
	}
	oldMount, ok := extenderTransferMount(op.SourcePath, oldDestination)
	if !ok || filepath.Clean(oldMount) != filepath.Clean(volume.MountPath) {
		return fmt.Errorf("Extender destination is not on the selected mounted volume")
	}
	newDestination := destinationPath(op.SourcePath, volume)
	if filepath.Clean(newDestination) == filepath.Clean(oldDestination) {
		return fmt.Errorf("source and destination layouts are identical")
	}
	if _, err := os.Lstat(newDestination); err == nil {
		return fmt.Errorf("antmove destination already exists: %s", newDestination)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := activeProcessFor(Item{Name: op.Name, SourcePath: op.SourcePath, Category: op.Category, Owner: op.Name}); err != nil {
		return err
	}
	digest, err := hashTree(oldDestination)
	if err != nil {
		return fmt.Errorf("verify old data before move: %w", err)
	}
	localPrivileged := directoryNeedsPrivilege(filepath.Dir(op.SourcePath))
	if err := ensurePrivilege(filepath.Dir(op.SourcePath), localPrivileged); err != nil {
		return err
	}
	volumePrivileged := destinationParentNeedsPrivilege(filepath.Dir(newDestination), volume.MountPath)
	if err := ensurePrivilege(volume.MountPath, volumePrivileged); err != nil {
		return err
	}
	if err := ensureSafeDestinationParent(volume.MountPath, filepath.Dir(newDestination)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newDestination), 0o755); err != nil {
		return fmt.Errorf("create antmove destination: %w", err)
	}
	op.PreviousDestinationPath = oldDestination
	op.DestinationPath = newDestination
	op.VolumeMountPath = volume.MountPath
	op.VerificationSHA256 = digest
	op.Phase, op.Error = "migrating", ""
	if err := saveOperation(op); err != nil {
		return err
	}
	if err := renamePath(oldDestination, newDestination, volumePrivileged); err != nil {
		op.DestinationPath, op.PreviousDestinationPath = oldDestination, ""
		op.VerificationSHA256, op.Phase, op.Error = "", "completed", err.Error()
		_ = saveOperation(op)
		return err
	}
	if err := verifyTreeDigest(newDestination, digest); err != nil {
		rollbackErr := renamePath(newDestination, oldDestination, volumePrivileged)
		if rollbackErr == nil {
			op.DestinationPath, op.PreviousDestinationPath = oldDestination, ""
			op.VerificationSHA256, op.Phase, op.Error = "", "completed", err.Error()
			_ = saveOperation(op)
		} else {
			op.Phase, op.Error = "migrating", err.Error()+"; rollback failed: "+rollbackErr.Error()
			_ = saveOperation(op)
		}
		return err
	}
	if err := replaceTransferLink(op.SourcePath, oldDestination, newDestination, op.ID, localPrivileged); err != nil {
		rollbackErr := renamePath(newDestination, oldDestination, volumePrivileged)
		if rollbackErr == nil {
			op.DestinationPath, op.PreviousDestinationPath = oldDestination, ""
			op.VerificationSHA256, op.Phase, op.Error = "", "completed", err.Error()
			_ = saveOperation(op)
		} else {
			op.Phase, op.Error = "migrating", err.Error()+"; rollback failed: "+rollbackErr.Error()
			_ = saveOperation(op)
		}
		return err
	}
	op.PreviousDestinationPath, op.VerificationSHA256 = "", ""
	op.Phase, op.Error = "completed", ""
	return saveOperation(op)
}

func destinationParentNeedsPrivilege(parent, mount string) bool {
	current := filepath.Clean(parent)
	for {
		if _, err := os.Stat(current); err == nil {
			return directoryNeedsPrivilege(current)
		}
		if current == filepath.Clean(mount) || filepath.Dir(current) == current {
			return directoryNeedsPrivilege(current)
		}
		current = filepath.Dir(current)
	}
}

func replaceTransferLink(source, expectedCurrent, destination, id string, privileged bool) error {
	current, err := os.Readlink(source)
	if err != nil || filepath.Clean(current) != filepath.Clean(expectedCurrent) {
		return fmt.Errorf("original Extender link changed during migration")
	}
	link := source + ".antmove-migration-" + id
	if _, err := os.Lstat(link); err == nil {
		return fmt.Errorf("temporary link already exists: %s", link)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := symlinkPath(destination, link, privileged); err != nil {
		return err
	}
	if privileged {
		if err := runPrivileged("/bin/mv", "-h", link, source); err != nil {
			_ = runPrivileged("/bin/rm", "-f", link)
			return err
		}
		return nil
	}
	if err := os.Rename(link, source); err != nil {
		_ = os.Remove(link)
		return err
	}
	return nil
}

func hashTree(root string) (string, error) {
	hasher := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
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
		mode := info.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(hasher, "L\x00%s\x00%s\x00", rel, target)
		case mode.IsDir():
			_, _ = fmt.Fprintf(hasher, "D\x00%s\x00%o\x00", rel, mode.Perm())
		case mode.IsRegular():
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			content := sha256.New()
			if _, err := io.Copy(content, file); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(hasher, "F\x00%s\x00%o\x00%d\x00%x\x00", rel, mode.Perm(), info.Size(), content.Sum(nil))
		default:
			return fmt.Errorf("unsupported special file at %s", path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyTreeDigest(root, expected string) error {
	actual, err := hashTree(root)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 verification failed for %s", root)
	}
	return nil
}

func recoverExtenderMigration(op Operation) (string, error) {
	oldDestination, newDestination := op.PreviousDestinationPath, op.DestinationPath
	if oldDestination == "" || op.VerificationSHA256 == "" {
		return "", fmt.Errorf("migration journal is incomplete; manual review required")
	}
	currentLink, linkErr := os.Readlink(op.SourcePath)
	oldExists := pathExists(oldDestination)
	newExists := pathExists(newDestination)
	if newExists {
		if err := verifyTreeDigest(newDestination, op.VerificationSHA256); err != nil {
			return "", err
		}
		if linkErr == nil && filepath.Clean(currentLink) == filepath.Clean(oldDestination) {
			privileged := directoryNeedsPrivilege(filepath.Dir(op.SourcePath))
			if err := ensurePrivilege(filepath.Dir(op.SourcePath), privileged); err != nil {
				return "", err
			}
			if err := replaceTransferLink(op.SourcePath, oldDestination, newDestination, op.ID, privileged); err != nil {
				return "", err
			}
		} else if linkErr != nil || filepath.Clean(currentLink) != filepath.Clean(newDestination) {
			return "", fmt.Errorf("source link no longer matches either migration path")
		}
		op.PreviousDestinationPath, op.VerificationSHA256 = "", ""
		op.Phase, op.Error = "completed", ""
		if err := saveOperation(op); err != nil {
			return "", err
		}
		return op.Name + ": verified and completed antmove path migration", nil
	}
	if oldExists && linkErr == nil && filepath.Clean(currentLink) == filepath.Clean(oldDestination) {
		if err := verifyTreeDigest(oldDestination, op.VerificationSHA256); err != nil {
			return "", err
		}
		op.DestinationPath, op.PreviousDestinationPath = oldDestination, ""
		op.VerificationSHA256, op.Phase, op.Error = "", "completed", ""
		if err := saveOperation(op); err != nil {
			return "", err
		}
		return op.Name + ": migration had not started; Extender path remains active", nil
	}
	return "", fmt.Errorf("migration paths are inconsistent; manual review required")
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func checkMigrationVolume(volume Volume) error {
	if !volume.Writable || volume.Network || !volume.Physical {
		return fmt.Errorf("disk %s is not eligible for migration", volume.Name)
	}
	var stats unix.Statfs_t
	if err := unix.Statfs(volume.MountPath, &stats); err != nil {
		return err
	}
	return nil
}
