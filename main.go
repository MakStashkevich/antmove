package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var terminalInput = bufio.NewReader(os.Stdin)
var startupNotice string

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	if autoInstallOnFirstRun(command) {
		installed, pathChanged, err := installSelf()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Automatic installation skipped:", err)
		} else if installed || pathChanged {
			startupNotice = fmt.Sprintf("Installed antmove and am in %s.", installDirectoryLabel())
			if pathChanged {
				startupNotice += " Reopen Terminal to refresh PATH."
			}
			if command != "" {
				fmt.Fprintln(os.Stderr, startupNotice)
			}
		}
	}
	if len(args) == 0 {
		return startTUI("home", false)
	}
	if command == "--help" || command == "-h" || command == "help" {
		printHelp()
		return nil
	}
	if command == "--version" || command == "version" {
		printVersion()
		return nil
	}
	switch command {
	case "scan":
		items, err := scanItems()
		if err != nil {
			return err
		}
		printItems(items)
		return nil
	case "status":
		return printStatus()
	case "disks":
		return runDisks(args[1:])
	case "move":
		return runMove(args[1:])
	case "restore":
		return runRestore(args[1:])
	case "recover":
		return runRecover()
	case "migrate":
		return runMigrate(args[1:])
	case "install":
		installed, pathChanged, err := installSelf()
		if err != nil {
			return err
		}
		if installed {
			fmt.Printf("Installed antmove and am in %s\n", installDirectoryLabel())
		} else {
			fmt.Printf("antmove and am are already installed in %s\n", installDirectoryLabel())
		}
		if pathChanged {
			fmt.Println("PATH was added to your shell startup file. Reopen Terminal to refresh PATH.")
		}
		return nil
	case "uninstall":
		return runUninstall()
	default:
		return fmt.Errorf("unknown command %q; run am --help", command)
	}
}

func runDisks(args []string) error {
	fs := flag.NewFlagSet("disks", flag.ContinueOnError)
	testPath := fs.String("test", "", "run a read/write benchmark on the selected mount path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *testPath != "" {
		result, err := testVolume(*testPath)
		if err != nil {
			return err
		}
		fmt.Printf("Speed test complete: read %.0f MB/s, write %.0f MB/s\n", result.ReadMBps, result.WriteMBps)
		return nil
	}
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	if len(volumes) == 0 {
		fmt.Println("No eligible physical external disks found.")
		return nil
	}
	for _, volume := range volumes {
		fmt.Printf("%s\n  %s\n  Games:      %s\n  Office apps: %s\n  Messengers: %s\n", volumeMessage(volume), volume.MountPath,
			gameSuitability(volume), appSuitability(volume), messengerSuitability(volume))
	}
	return nil
}

type countFlag int

func (c *countFlag) String() string { return fmt.Sprint(int(*c)) }
func (c *countFlag) Set(value string) error {
	if value != "" {
		return errors.New("--confirm-caution does not take a value")
	}
	*c = *c + 1
	return nil
}

func runMove(args []string) error {
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	disk := fs.String("disk", "", "destination volume mount path")
	dryRun := fs.Bool("dry-run", false, "show the transfer plan without changing files")
	yes := fs.Bool("yes", false, "confirm a transfer on a recommended disk")
	var cautionConfirm countFlag
	fs.Var(&cautionConfirm, "confirm-caution", "acknowledge a non-recommended disk (repeat twice)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ids := fs.Args()
	if len(ids) == 0 {
		return startTUI("items", *dryRun)
	}
	if *disk == "" {
		return errors.New("--disk is required when item IDs are provided")
	}
	items, err := scanItems()
	if err != nil {
		return err
	}
	selected, err := selectItems(items, ids)
	if err != nil {
		return err
	}
	volumes, err := scanVolumes()
	if err != nil {
		return err
	}
	volume := findVolume(volumes, "", *disk)
	if volume == nil {
		return fmt.Errorf("destination disk not found: %s", *disk)
	}
	if !*dryRun {
		if volume.Rating == RatingCaution {
			if cautionConfirm >= 2 {
				// Both non-interactive acknowledgements were supplied explicitly.
			} else if isTerminal() {
				if err := confirmCaution(*volume); err != nil {
					return err
				}
			} else if cautionConfirm < 2 {
				return errors.New("non-recommended disks require --confirm-caution twice")
			}
		} else if !*yes {
			if !isTerminal() {
				return errors.New("non-interactive transfers on recommended disks require --yes")
			}
			if !confirm("Move selected items to " + volume.Name + "? [y/N] ") {
				return errors.New("cancelled")
			}
		}
	}
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	for _, item := range selected {
		op, err := moveItem(item, *volume, *dryRun)
		if err != nil {
			fmt.Printf("✗ %s: %v\n", item.Name, err)
			if !*dryRun {
				return err
			}
			continue
		}
		fmt.Printf("✓ %s · %s · %s\n", item.Name, formatBytes(item.SizeBytes), op.DestinationPath)
	}
	return nil
}

func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show the restore plan without changing files")
	yes := fs.Bool("yes", false, "confirm restore")
	if err := fs.Parse(args); err != nil {
		return err
	}
	operations, err := loadOperations()
	if err != nil {
		return err
	}
	ids := fs.Args()
	if len(ids) == 0 {
		return startTUI("restore", *dryRun)
	}
	if _, err := scanItems(); err != nil {
		return err
	}
	operations, err = loadOperations()
	if err != nil {
		return err
	}
	selected := make([]Operation, 0, len(ids))
	for _, id := range ids {
		found := false
		for _, op := range operations {
			if (op.Phase == "completed" || op.Phase == "cleanup_pending") && (op.ID == id || op.ItemID == id) {
				selected = append(selected, op)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("moved item not found: %s", id)
		}
	}
	if !*dryRun && !*yes {
		if !isTerminal() {
			return errors.New("non-interactive restores require --yes")
		}
		if !confirm("Restore selected items to their original locations? [y/N] ") {
			return errors.New("cancelled")
		}
	}
	if *dryRun {
		for _, op := range selected {
			fmt.Printf("Would restore %s from %s to %s\n", op.Name, op.DestinationPath, op.SourcePath)
		}
		return nil
	}
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	for _, op := range selected {
		if err := restoreOperation(op); err != nil {
			fmt.Printf("✗ %s: %v\n", op.Name, err)
			return err
		}
		fmt.Printf("✓ Restored %s\n", op.Name)
	}
	return nil
}

func runRecover() error {
	if isTerminal() && !confirm("Recover interrupted antmove operations? [y/N] ") {
		return errors.New("cancelled")
	}
	unlock, err := acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	results, err := recoverOperations()
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("No interrupted operations need recovery.")
		return nil
	}
	for _, result := range results {
		fmt.Println(result)
	}
	return nil
}

func printStatus() error {
	if _, err := scanItems(); err != nil {
		return err
	}
	operations, err := loadOperations()
	if err != nil {
		return err
	}
	volumes, _ := scanVolumes()
	if len(operations) == 0 {
		fmt.Println("No antmove operations recorded. Run `am scan` to find candidates.")
		return nil
	}
	for _, op := range operations {
		state := op.Phase
		if op.Phase == "completed" && findOperationVolume(volumes, op) == nil {
			state = "disk offline"
		}
		row := fmt.Sprintf("%-16s %s · %s · %s", state, op.Name, formatBytes(op.SizeBytes), op.SourcePath)
		if stdoutIsTerminal() {
			if state == "completed" {
				row = tuiSuccessStyle.Render(row)
			} else if state == "disk offline" {
				row = tuiWarningStyle.Render(row)
			}
		}
		fmt.Println(row)
		fmt.Printf("  ID: %s\n", op.ItemID)
		fmt.Printf("  Disk: %s · %s\n", op.VolumeName, op.DestinationPath)
		if op.Origin != "" {
			fmt.Printf("  Imported from: %s\n", op.Origin)
		}
		if op.Error != "" {
			fmt.Printf("  Note: %s\n", op.Error)
		}
	}
	return nil
}

func printItems(items []Item) {
	if len(items) == 0 {
		fmt.Println("No transfer candidates found.")
		return
	}
	fmt.Printf("%-12s %-24s %-20s %-14s %10s  %s\n", "ID", "NAME", "CATEGORY", "STATUS", "SIZE", "PATH")
	for _, item := range items {
		row := fmt.Sprintf("%-12s %-24s %-20s %-14s %10s  %s", item.ID, truncate(item.Name, 23),
			item.Category, item.Status, formatBytes(item.SizeBytes), item.SourcePath)
		if stdoutIsTerminal() {
			switch item.Status {
			case StatusMoved:
				row = tuiSuccessStyle.Render(row)
			case StatusUnavailable, StatusDiskOffline:
				row = tuiWarningStyle.Render(row)
			}
		}
		fmt.Println(row)
		if item.Owner != "" {
			fmt.Printf("  App: %s\n", item.Owner)
		}
		if item.Reason != "" {
			fmt.Printf("  Note: %s\n", item.Reason)
		}
	}
}

func selectItems(items []Item, ids []string) ([]Item, error) {
	selected := make([]Item, 0, len(ids))
	for _, id := range ids {
		found := false
		for _, item := range items {
			if item.ID == id || filepath.Clean(item.SourcePath) == filepath.Clean(id) {
				selected = append(selected, item)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("candidate not found: %s", id)
		}
	}
	return selected, nil
}

func isTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	answer := readLine()
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

func confirmCaution(volume Volume) error {
	return confirmCautionUsing(volume, confirm, readLine)
}

func confirmCautionUsing(volume Volume, confirmFn func(string) bool, readFn func() string) error {
	fmt.Printf("WARNING: %s is not recommended (%s). It may be slow or lose macOS metadata.\n", volume.Name, volume.FileSystem)
	if !confirmFn("First confirmation: continue despite this warning? [y/N] ") {
		return errors.New("cancelled")
	}
	fmt.Printf("Second confirmation: type the exact disk name %q: ", volume.Name)
	answer := readFn()
	if answer != volume.Name {
		return errors.New("disk name did not match; cancelled")
	}
	return nil
}

func readLine() string {
	line, _ := terminalInput.ReadString('\n')
	return strings.TrimSpace(line)
}

func printHelp() {
	colorOutput := stdoutIsTerminal()
	for i := range antLogo {
		if colorOutput {
			fmt.Println(styledBrandHeaderLine(i))
		} else {
			fmt.Println(brandHeaderLine(i))
		}
	}
	fmt.Println()
	printHelpSection("COMMANDS", [][2]string{
		{"am", "Open the interactive menu"},
		{"am scan", "Find apps, Apple media, backups, and developer data"},
		{"am move", "Move selected candidates to an external disk"},
		{"am restore", "Restore moved data to its original path"},
		{"am status", "Show transfer history and disk availability"},
		{"am disks", "List external disks and suitability"},
		{"am recover", "Recover interrupted transfers"},
		{"am migrate", "Move existing Extender transfers into antmove storage"},
		{"am install", "Install or repair antmove and am"},
		{"am uninstall", "Remove antmove and its local state"},
		{"am --help", "Show this command list"},
		{"am --version", "Show system and version information"},
	})
	fmt.Println()
	printHelpSection("OPTIONS", [][2]string{
		{"--disk PATH", "Destination mount path for am move"},
		{"--dry-run", "Preview a move or restore without changing files"},
		{"--yes", "Confirm a recommended-disk move or restore"},
		{"--confirm-caution", "Acknowledge a caution disk; repeat twice"},
		{"--test PATH", "Measure read and write speed for a disk"},
		{"[ID…]", "Limit move or restore to selected items; omit for menu"},
	})
	fmt.Println()
	printHelpSection("EXAMPLES", [][2]string{
		{"am scan", "List available candidates"},
		{"am disks --test /Volumes/WorkSSD", "Measure external-disk speed"},
		{"am move --disk /Volumes/WorkSSD --dry-run ID", "Preview a transfer"},
		{"am restore --dry-run ID", "Preview a restore"},
		{"am migrate --dry-run", "Preview Extender path migration"},
	})
	fmt.Println()
	fmt.Println("Moves to caution-rated disks require two separate confirmations. Read-only, network, system, and unavailable disks cannot be used as destinations.")
}

func printHelpSection(title string, entries [][2]string) {
	if stdoutIsTerminal() {
		fmt.Println(tuiHeadingStyle.Render(title))
	} else {
		fmt.Println(title)
	}
	columnWidth := 31
	for _, entry := range entries {
		columnWidth = maxInt(columnWidth, lipgloss.Width(entry[0]))
	}
	for _, entry := range entries {
		label := entry[0]
		padding := columnWidth - lipgloss.Width(label) + 1
		command := label
		if stdoutIsTerminal() {
			command = tuiSuccessStyle.Render(command)
		}
		fmt.Printf("  %s%s%s\n", command, strings.Repeat(" ", padding), entry[1])
	}
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func printVersion() {
	fmt.Printf("antmove version %s\n", version)
	fmt.Printf("macOS: %s\n", systemCommandValue("/usr/bin/sw_vers", "-productVersion"))
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)
	fmt.Printf("Kernel: %s\n", systemCommandValue("/usr/bin/uname", "-r"))
	fmt.Printf("SIP: %s\n", sipStatus())
	fmt.Printf("Disk Free: %s\n", rootDiskFree())
	fmt.Printf("Install: %s\n", installMethod())
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "Unknown"
	}
	fmt.Printf("Shell: %s\n", shell)
}

func systemCommandValue(path string, args ...string) string {
	output, err := commandOutput(path, args...)
	if err != nil {
		return "Unknown"
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "Unknown"
	}
	return value
}

func sipStatus() string {
	output, err := commandOutput("/usr/bin/csrutil", "status")
	if err != nil {
		return "Unknown"
	}
	status := strings.ToLower(string(output))
	if strings.Contains(status, "disabled") {
		return "Disabled"
	}
	if strings.Contains(status, "enabled") {
		return "Enabled"
	}
	return "Unknown"
}

func rootDiskFree() string {
	output, err := commandOutput("/bin/df", "-k", "/")
	if err != nil {
		return "Unknown"
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return "Unknown"
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return "Unknown"
	}
	availableKB, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return "Unknown"
	}
	freeGB := availableKB * 1024 / 1_000_000_000
	return fmt.Sprintf("%.2f GB", freeGB)
}

func installMethod() string {
	executable, err := os.Executable()
	if err != nil {
		return "Standalone"
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	if absolute, err := filepath.Abs(executable); err == nil {
		executable = absolute
	}
	home, _ := os.UserHomeDir()
	binDirectory, err := filepath.Abs(installBinDirectory(home))
	if err == nil && filepath.Clean(executable) == filepath.Join(filepath.Clean(binDirectory), "antmove") {
		if os.Getenv("ANTMOVE_INSTALL_DIR") == "" {
			return "User (~/.local/bin)"
		}
		return "User (" + installDirectoryLabel() + ")"
	}
	if strings.Contains(executable, "/Cellar/antmove/") {
		return "Homebrew"
	}
	return "Standalone"
}

func startTUI(page string, dryRun bool) error {
	if !isTerminal() {
		return errors.New("interactive mode needs a terminal; use `am scan`, `am status`, or explicit command options")
	}
	model, err := newTUIModel(page, dryRun)
	if err != nil {
		return err
	}
	program := teaNewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	if final, ok := finalModel.(tuiModel); ok {
		switch final.quitAction {
		case "help":
			printHelp()
		case "version":
			printVersion()
		}
	}
	return nil
}

func packageHint(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return path
}
