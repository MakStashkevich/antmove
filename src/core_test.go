package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

func TestRateVolume(t *testing.T) {
	ssd, hdd := true, false
	cases := []struct {
		name string
		vol  Volume
		want VolumeRating
	}{
		{"APFS SSD", Volume{FileSystem: "apfs", Physical: true, Writable: true, SolidState: &ssd}, RatingBest},
		{"APFS HDD", Volume{FileSystem: "apfs", Physical: true, Writable: true, SolidState: &hdd}, RatingGood},
		{"APFS unknown medium", Volume{FileSystem: "apfs", Physical: true, Writable: true}, RatingGood},
		{"HFS+", Volume{FileSystem: "hfs+", Physical: true, Writable: true}, RatingGood},
		{"ExFAT", Volume{FileSystem: "exfat", Physical: true, Writable: true}, RatingCaution},
		{"unknown format", Volume{FileSystem: "mystery", Physical: true, Writable: true}, RatingCaution},
		{"read-only", Volume{FileSystem: "apfs", Physical: true, Writable: false, SolidState: &ssd}, RatingUnavailable},
		{"network", Volume{FileSystem: "apfs", Physical: true, Writable: true, Network: true, SolidState: &ssd}, RatingUnavailable},
		{"non-physical", Volume{FileSystem: "apfs", Writable: true, SolidState: &ssd}, RatingUnavailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := rateVolume(test.vol); got != test.want {
				t.Fatalf("rateVolume() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiskSpeedSuitabilityThresholds(t *testing.T) {
	cases := []struct {
		mbps       float64
		games      string
		apps       string
		messengers string
	}{
		{149, "Not recommended for games", "Good for applications", "Excellent for messengers"},
		{350, "Good for most games", "Excellent for applications", "Excellent for messengers"},
		{19, "Not recommended for games", "May feel slow", "May feel slow"},
		{50, "Not recommended for games", "Suitable for light applications", "Good for messengers"},
	}
	for _, test := range cases {
		volume := Volume{ReadMBps: test.mbps}
		if got := gameSuitability(volume); got != test.games {
			t.Errorf("at %.0f MB/s game suitability = %q, want %q", test.mbps, got, test.games)
		}
		if got := appSuitability(volume); got != test.apps {
			t.Errorf("at %.0f MB/s app suitability = %q, want %q", test.mbps, got, test.apps)
		}
		if got := messengerSuitability(volume); got != test.messengers {
			t.Errorf("at %.0f MB/s messenger suitability = %q, want %q", test.mbps, got, test.messengers)
		}
	}
}

func TestRequiredCapacity(t *testing.T) {
	if got := requiredCapacity(0); got != 1024*1024*1024 {
		t.Fatalf("empty transfer reserve = %d, want 1 GiB", got)
	}
	if got := requiredCapacity(10_000_000_000); got != 11_073_741_824 {
		t.Fatalf("10 GB transfer reserve = %d, want 10 GB plus 1 GiB", got)
	}
	if got := requiredCapacity(int64(^uint64(0) >> 1)); got != int64(^uint64(0)>>1) {
		t.Fatalf("overflow must saturate at MaxInt64, got %d", got)
	}
}

func TestPlistMapReadsStandardPlist(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>answer</key><string>yes</string></dict></plist>`)
	values, err := plistMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringProperty(values, "answer"); got != "yes" {
		t.Fatalf("answer = %q, want yes", got)
	}
}

func TestVerifyTreeHandlesUnicodeSpacesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source with spaces")
	destination := filepath.Join(root, "destination copy")
	nested := filepath.Join(source, "Приложение.app", "Contents", "Resources", "данные с пробелами.txt")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("unicode content: Привет, мир\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("данные с пробелами.txt", filepath.Join(filepath.Dir(nested), "shortcut")); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/usr/bin/ditto", "--rsrc", "--extattr", "--acl", source, destination)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ditto copy failed: %v: %s", err, output)
	}
	if err := verifyTree(source, destination); err != nil {
		t.Fatalf("verifyTree() rejected identical Unicode tree: %v", err)
	}
	if err := os.WriteFile(nested, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyTree(source, destination); err == nil {
		t.Fatal("verifyTree() accepted content with a different SHA-256")
	}
}

func TestEnsureSafeDestinationParentRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	mount := filepath.Join(root, "external disk")
	other := filepath.Join(root, "outside")
	if err := os.MkdirAll(mount, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(mount, "antmove"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(mount, "antmove", "Applications")); err != nil {
		t.Fatal(err)
	}
	if err := ensureSafeDestinationParent(mount, filepath.Join(mount, "antmove", "Applications", "Demo.app")); err == nil {
		t.Fatal("destination symlink was accepted")
	}
	entries, err := os.ReadDir(other)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsafe path unexpectedly wrote outside the selected mount: %v", entries)
	}
}

func TestCautionDiskRequiresBothAcknowledgements(t *testing.T) {
	volume := Volume{Name: "Shared Disk", FileSystem: "exfat"}
	called := false
	confirmYes := func(string) bool { return true }
	if err := confirmCautionUsing(volume, func(string) bool { return false }, func() string { return volume.Name }); err == nil {
		t.Fatal("caution disk accepted without first acknowledgement")
	}
	if err := confirmCautionUsing(volume, confirmYes, func() string { return "wrong" }); err == nil {
		t.Fatal("caution disk accepted without typing the exact disk name")
	}
	if err := confirmCautionUsing(volume, confirmYes, func() string { called = true; return volume.Name }); err != nil {
		t.Fatalf("valid two-step confirmation was rejected: %v", err)
	}
	if !called {
		t.Fatal("second confirmation was not requested")
	}
}

func TestCautionFlagMustBeRepeatedWithoutValue(t *testing.T) {
	var flag countFlag
	if err := flag.Set(""); err != nil {
		t.Fatal(err)
	}
	if err := flag.Set(""); err != nil {
		t.Fatal(err)
	}
	if flag != 2 {
		t.Fatalf("two acknowledgements recorded as %d", flag)
	}
	if err := flag.Set("yes"); err == nil {
		t.Fatal("confirmation flag accepted an arbitrary value")
	}
}

func TestFirstRunInstallerCopiesBinaryAndCreatesAlias(t *testing.T) {
	binDirectory := filepath.Join(t.TempDir(), "local bin")
	t.Setenv("ANTMOVE_INSTALL_DIR", binDirectory)
	installed, pathChanged, err := installSelf()
	if err != nil {
		t.Fatal(err)
	}
	if !installed || pathChanged {
		t.Fatalf("installSelf() = installed %v, path changed %v", installed, pathChanged)
	}
	info, err := os.Stat(filepath.Join(binDirectory, "antmove"))
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary is missing or not executable: %v", err)
	}
	target, err := os.Readlink(filepath.Join(binDirectory, "am"))
	if err != nil || target != "antmove" {
		t.Fatalf("am alias target = %q, %v", target, err)
	}
}

func TestEnsureShellPathIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/zsh")
	profile := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(profile, []byte("export EDITOR=vim\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := ensureShellPath(home, filepath.Join(home, ".local", "bin"))
	if err != nil || !changed {
		t.Fatalf("first PATH update = %v, %v", changed, err)
	}
	changed, err = ensureShellPath(home, filepath.Join(home, ".local", "bin"))
	if err != nil || changed {
		t.Fatalf("repeated PATH update = %v, %v", changed, err)
	}
	contents, err := os.ReadFile(profile)
	if err != nil || strings.Count(string(contents), "antmove: keep the am command") != 1 {
		t.Fatalf("shell profile update is missing or duplicated: %v", err)
	}
}

func TestInstallerRefusesToReplaceExistingAmCommand(t *testing.T) {
	binDirectory := t.TempDir()
	alias := filepath.Join(binDirectory, "am")
	if err := os.WriteFile(alias, []byte("user command"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureAlias(alias); err == nil {
		t.Fatal("installer replaced an unrelated am command")
	}
	contents, err := os.ReadFile(alias)
	if err != nil || string(contents) != "user command" {
		t.Fatalf("existing am command was changed: %q, %v", contents, err)
	}
}

func TestFirstInvocationAutoInstallsExceptExplicitInstallCommand(t *testing.T) {
	if !autoInstallOnFirstRun("--help") || !autoInstallOnFirstRun("--version") || !autoInstallOnFirstRun("scan") {
		t.Fatal("first invocation should install for normal commands and help/version")
	}
	if autoInstallOnFirstRun("install") || autoInstallOnFirstRun("uninstall") {
		t.Fatal("install and uninstall commands should not trigger auto-install")
	}
}

func TestUninstallRequiresBothConfirmations(t *testing.T) {
	reads := 0
	if err := confirmUninstallUsing(func(string) bool { return false }, func() string { reads++; return uninstallConfirmation }); err == nil {
		t.Fatal("uninstall proceeded without the first confirmation")
	}
	if reads != 0 {
		t.Fatal("second confirmation was requested after the first was declined")
	}
	if err := confirmUninstallUsing(func(string) bool { return true }, func() string { return "wrong" }); err == nil {
		t.Fatal("uninstall proceeded with an incorrect second confirmation")
	}
	if err := confirmUninstallUsing(func(string) bool { return true }, func() string { return uninstallConfirmation }); err != nil {
		t.Fatalf("valid confirmations were rejected: %v", err)
	}
}

func TestUninstallRemovesOnlyAntmoveFilesAndManagedPathBlock(t *testing.T) {
	home := t.TempDir()
	binDirectory := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDirectory, "antmove"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("antmove", filepath.Join(binDirectory, "am")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDirectory, "other-tool"), []byte("keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, ".zshrc")
	profileContents := "export EDITOR=vim\n" + managedPathBlock + "export CUSTOM=value\n"
	if err := os.WriteFile(profile, []byte(profileContents), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(home, "Library", "Application Support", "antmove")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", "/bin/zsh")
	if err := uninstallAntmove(home, binDirectory, stateDir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(binDirectory, "antmove"), filepath.Join(binDirectory, "am"), stateDir} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, got err=%v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(binDirectory, "other-tool")); err != nil {
		t.Errorf("unrelated executable was removed: %v", err)
	}
	contents, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "export EDITOR=vim\n\nexport CUSTOM=value\n" {
		t.Errorf("profile after uninstall = %q", got)
	}
}

func TestTUIViewWrapsLongDiskRowsToTerminalWidth(t *testing.T) {
	line := "› Рабочий дисковый том · apfs SSD · USB · Recommended · read 678 / write 757 MB/s · free 613.5 GB"
	wrapped := wrapTerminalView(line, 42)
	for _, row := range strings.Split(wrapped, "\n") {
		if width := runewidth.StringWidth(row); width > 42 {
			t.Errorf("wrapped row uses %d cells, want at most 42: %q", width, row)
		}
	}
	if !strings.Contains(strings.ReplaceAll(wrapped, "\n", " "), "free 613.5 GB") {
		t.Fatalf("wrapped output lost the disk free-space details: %q", wrapped)
	}
}

func TestHomeMenuListsPrimaryCommands(t *testing.T) {
	model, err := newTUIModel("home", false)
	if err != nil {
		t.Fatal(err)
	}
	view := model.View()
	for _, command := range []string{"Move data", "Scan candidates", "Restore moved data", "External disks", "Operation status", "Recover interrupted transfer", "Uninstall antmove"} {
		if !strings.Contains(view, command) {
			t.Errorf("home menu does not list %q", command)
		}
	}
}

func TestReadLinePreservesDiskNamesWithSpaces(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("T7 Рабочий SSD\n"))
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(line); got != "T7 Рабочий SSD" {
		t.Fatalf("read disk name = %q", got)
	}
}

func TestMoveAndRestoreEndToEnd(t *testing.T) {
	if os.Getenv("ANTMOVE_E2E") != "1" {
		t.Skip("set ANTMOVE_E2E=1 to exercise a full transfer on a mounted external disk")
	}
	volumes, err := scanVolumes()
	if err != nil {
		t.Fatal(err)
	}
	var destination *Volume
	for i := range volumes {
		if volumes[i].Rating != RatingUnavailable {
			destination = &volumes[i]
			break
		}
	}
	if destination == nil {
		t.Skip("no writable external disk is available")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	stateDirectoryOverride = t.TempDir()
	t.Cleanup(func() { stateDirectoryOverride = "" })
	parent, err := os.MkdirTemp(home, ".antmove-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(parent) })
	source := filepath.Join(parent, "Unicode Demo.app")
	resource := filepath.Join(source, "Contents", "Resources", "hello world.txt")
	if err := os.MkdirAll(filepath.Dir(resource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("Привет, antmove\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	item, ok := makeItem(source, filepath.Base(source), CategoryApplication, "", nil)
	if !ok {
		t.Fatal("could not build transfer candidate")
	}
	op, err := moveItem(item, *destination, false)
	if err != nil {
		t.Fatal(err)
	}
	restored := false
	t.Cleanup(func() {
		if !restored && (op.Phase == "completed" || op.Phase == "cleanup_pending") {
			if restoreErr := restoreOperation(op); restoreErr != nil {
				t.Errorf("cleanup restore failed: %v", restoreErr)
				return
			}
		}
		if info, statErr := os.Lstat(source); statErr == nil && info.Mode()&os.ModeSymlink == 0 {
			_ = os.RemoveAll(op.DestinationPath)
			_ = os.RemoveAll(filepath.Dir(op.DestinationPath))
			_ = removeOperation(op)
		}
	})
	info, err := os.Lstat(source)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("moved source was not replaced with a symbolic link: %v", err)
	}
	movedResource := filepath.Join(op.DestinationPath, "Contents", "Resources", "hello world.txt")
	movedData, err := os.ReadFile(movedResource)
	if err != nil || string(movedData) != "Привет, antmove\n" {
		t.Fatalf("moved content differs: %q, %v", movedData, err)
	}
	if err := restoreOperation(op); err != nil {
		t.Fatal(err)
	}
	restored = true
	info, err = os.Lstat(source)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("restore did not recreate the original directory: %v", err)
	}
	data, err := os.ReadFile(resource)
	if err != nil || string(data) != "Привет, antmove\n" {
		t.Fatalf("restored content differs: %q, %v", data, err)
	}
}

func TestRecoveryCleansInterruptedLinkingCopy(t *testing.T) {
	if os.Getenv("ANTMOVE_E2E") != "1" {
		t.Skip("set ANTMOVE_E2E=1 to exercise recovery on a mounted external disk")
	}
	volumes, err := scanVolumes()
	if err != nil {
		t.Fatal(err)
	}
	var destination *Volume
	for i := range volumes {
		if volumes[i].Rating != RatingUnavailable {
			destination = &volumes[i]
			break
		}
	}
	if destination == nil {
		t.Skip("no writable external disk is available")
	}
	stateDirectoryOverride = t.TempDir()
	t.Cleanup(func() { stateDirectoryOverride = "" })
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.MkdirTemp(home, ".antmove-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(parent)
		_ = os.RemoveAll(filepath.Join(destination.MountPath, "antmove", strings.TrimLeft(parent, string(filepath.Separator))))
	})
	source := filepath.Join(parent, "Recovery.app")
	resource := filepath.Join(source, "Contents", "Resources", "asset.txt")
	if err := os.MkdirAll(filepath.Dir(resource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("recovery fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: randomID(), ItemID: shortID(source), Name: "Recovery.app", Category: CategoryApplication,
		SourcePath: source, DestinationPath: destinationPath(source, *destination), VolumeUUID: destination.UUID,
		VolumeName: destination.Name, Phase: "linking", BackupPath: filepath.Join(parent, ".antmove-backup-recovery"),
		CreatedAt: time.Now()}
	if err := os.MkdirAll(filepath.Dir(op.DestinationPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := dittoCopy(source, op.DestinationPath, false); err != nil {
		t.Fatalf("prepare interrupted transfer fixture: %v: %s", err, output)
	}
	if err := saveOperation(op); err != nil {
		t.Fatal(err)
	}
	results, err := recoverOperations()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !strings.Contains(results[0], "original remains") {
		t.Fatalf("unexpected recovery result: %v", results)
	}
	if _, err := os.Lstat(op.DestinationPath); !os.IsNotExist(err) {
		t.Fatalf("interrupted duplicate was not cleaned: %v", err)
	}
	if _, err := os.Stat(resource); err != nil {
		t.Fatalf("original data did not remain in place: %v", err)
	}
	operations, err := loadOperations()
	if err != nil || len(operations) != 1 || operations[0].Phase != "failed" {
		t.Fatalf("recovery journal state = %+v, %v", operations, err)
	}
}
