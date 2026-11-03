package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const unassignedMinimumBytes int64 = 256 * 1024 * 1024
const candidateMinimumBytes int64 = 128 * 1024 * 1024

type appIdentity struct {
	Name       string
	BundleID   string
	Executable string
	Path       string
}

func scanItems() ([]Item, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	operations, err := loadOperations()
	if err != nil {
		return nil, err
	}
	bySource := make(map[string]Operation)
	for _, op := range operations {
		if op.Phase == "completed" || op.Phase == "cleanup_pending" || op.Phase == "recovery_required" {
			bySource[op.SourcePath] = op
		}
	}

	items := make(map[string]Item)
	apps := scanApplications(home, items, bySource)
	associated := scanAssociatedAppData(home, apps, items, bySource)
	scanDeveloperData(home, items, bySource)
	scanAIAssistantData(home, items, bySource)
	scanAppleMediaData(home, associated, items, bySource)
	scanUnassignedData(home, associated, items, bySource)
	scanContainerStorage(home, items)
	operations, err = importExtenderTransfers(items, operations)
	if err != nil {
		return nil, err
	}
	for _, op := range operations {
		if op.Phase != "completed" && op.Phase != "cleanup_pending" && op.Phase != "recovery_required" {
			continue
		}
		if _, exists := items[op.SourcePath]; exists {
			continue
		}
		item := Item{ID: op.ItemID, Name: op.Name, Category: op.Category, SourcePath: op.SourcePath,
			DestinationPath: op.DestinationPath, VolumeUUID: op.VolumeUUID, SizeBytes: op.SizeBytes}
		if _, err := os.Stat(op.DestinationPath); err == nil {
			item.Status = StatusMoved
		} else {
			item.Status, item.Reason = StatusDiskOffline, "The linked destination is currently unavailable."
		}
		items[op.SourcePath] = item
	}

	result := make([]Item, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	sortItems(result)
	return result, nil
}

// AI assistant state often contains conversation history, local indexes, and
// project context. Inspect direct children of the home and .config directories
// so a broad name heuristic cannot turn unrelated nested folders into candidates.
func scanAIAssistantData(home string, items map[string]Item, bySource map[string]Operation) {
	roots := []string{
		home,
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "state"),
		filepath.Join(home, "Library", "Application Support"),
	}
	for _, root := range roots {
		if hasSymlinkAncestor(root) {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if hasSymlinkAncestor(path) || entry.Type()&fs.ModeSymlink != 0 || !entry.IsDir() || !isAIAssistantDirectory(entry.Name()) {
				continue
			}
			if item, ok := makeItem(path, entry.Name(), CategoryAIData, "", bySource); ok {
				item.Reason = "AI assistant data may contain conversation history, project context, and local indexes. Quit the assistant and finish any active sync or generation before moving. Review the folder contents before moving."
				if item.RuntimeSockets > 0 {
					item.Reason += fmt.Sprintf(" Found %d Unix socket runtime endpoint(s); inactive sockets will be omitted from the copy, while active sockets block the move.", item.RuntimeSockets)
				}
				items[path] = item
			}
		}
	}
}

func isAIAssistantDirectory(name string) bool {
	n := strings.ToLower(strings.TrimPrefix(name, "."))
	n = strings.NewReplacer(" ", "-", "_", "-").Replace(n)
	known := []string{
		"codex", "agents", "claude", "claude-code", "cursor", "copilot", "github-copilot",
		"kilocode", "kilo", "hermes", "roo", "roo-code", "cline", "continue", "aider",
		"windsurf", "codeium", "gemini", "antigravity", "antigravity-ide", "qwen", "qwen-code",
		"opencode", "open-code", "goose", "amp", "zed", "kiro", "amazonq", "junie",
		"openclaw", "pi", "pi-coding-agent", "crush", "factory", "droid", "kimi", "trae",
		"warp", "replit", "void", "sourcegraph-cody", "cody", "tabby", "ollama", "lm-studio",
		"chatgpt", "openai", "jan", "anythingllm", "msty", "gpt4all", "perplexity",
	}
	for _, candidate := range known {
		if n == candidate || strings.HasPrefix(n, candidate+"-") || strings.HasPrefix(n, candidate+".") {
			return true
		}
	}
	// Catch common product variants without matching generic dotfolders.
	for _, marker := range []string{"copilot", "codex", "assistant", "claude", "llm", "kilocode", "hermes", "aider", "opencode", "antigravity", "qwen", "gemini", "openclaw", "windsurf", "cursor", "cline", "junie", "kiro"} {
		if strings.Contains(n, marker) {
			return true
		}
	}
	return false
}

func scanAppleMediaData(home string, associated map[string]bool, items map[string]Item, bySource map[string]Operation) {
	pictures := filepath.Join(home, "Pictures")
	if !hasSymlinkAncestor(pictures) {
		entries, err := os.ReadDir(pictures)
		if err == nil {
			for _, entry := range entries {
				if !strings.EqualFold(filepath.Ext(entry.Name()), ".photoslibrary") {
					continue
				}
				path := filepath.Join(pictures, entry.Name())
				if !entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
					continue
				}
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				addAppleManagedCandidate(path, name, CategoryAppleMedia, "Photos",
					"Apple Photos library. Quit Photos before moving. Apple recommends APFS or Mac OS Extended; after moving, open the library on the external disk and select it as the System Photo Library if you use iCloud Photos.",
					associated, items, bySource)
			}
		}
	}
	defaultPhotosPath := filepath.Join(pictures, "Photos Library.photoslibrary")
	if _, exists := items[defaultPhotosPath]; !exists {
		addAppleManagedCandidate(defaultPhotosPath, "Photos Library (default)", CategoryAppleMedia, "Photos",
			"Default Apple Photos library location. Quit Photos before moving. Apple recommends APFS or Mac OS Extended; after moving, open the library on the external disk and select it as the System Photo Library if you use iCloud Photos. For a new library, Photos may need to create it through its Option-open > Create New workflow.",
			associated, items, bySource)
	}

	musicCandidates := []struct {
		path   string
		name   string
		legacy bool
	}{
		{path: appleMusicMediaFolder(home), name: "Apple Music Media"},
		{path: filepath.Join(home, "Music", "iTunes", "iTunes Media"), name: "Legacy iTunes Media", legacy: true},
		{path: filepath.Join(home, "Music", "iTunes", "iTunes Music"), name: "Legacy iTunes Music", legacy: true},
	}
	for _, candidate := range musicCandidates {
		if candidate.legacy {
			if _, err := os.Lstat(candidate.path); os.IsNotExist(err) {
				continue
			}
		}
		note := "Apple Music media folder. Quit Music before moving. For future imports, check Music > Settings > Files > Media Location; keep the external disk connected while using this library."
		if candidate.legacy {
			note = "Legacy iTunes media folder. iTunes is not the current Mac music app; move this only if these existing files are still part of a library you use. Check Music > Settings > Files > Media Location and keep the external disk connected while using the library."
		}
		addAppleManagedCandidate(candidate.path, candidate.name, CategoryAppleMedia, "Music",
			note, associated, items, bySource)
	}

	// Finder and Apple Devices keep iPhone and iPad backups in MobileSync.
	backupPath := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	addAppleManagedCandidate(backupPath, "iPhone & iPad Backups", CategoryAppleBackup, "",
		"Finder/Apple Devices backup folder. Wait for any device backup or sync to finish before moving. If access is denied, give your terminal Full Disk Access and scan again.",
		associated, items, bySource)

	// The TV app exposes its Media folder location in Settings > Files. Use that
	// configured path rather than moving the enclosing Movies/TV library folder.
	// Download cache locations are app-managed and undocumented, so only surface
	// one if a known directory is actually present.
	tvMediaPath := appleTVMediaFolder(home)
	tvCandidates := []struct {
		path       string
		name       string
		note       string
		ifExisting bool
	}{
		{tvMediaPath, "Apple TV Media", "Apple TV media folder from TV > Settings > Files. Quit TV before moving. After moving, verify the Media folder location in TV settings; downloaded titles may be DRM protected and need the disk connected for playback.", false},
		{filepath.Join(home, "Library", "Containers", "com.apple.TV", "Data", "Library", "Caches", "com.apple.TV", "Downloads"), "Apple TV Downloads", "Apple TV app-managed download cache. Quit TV before moving. Downloads may be DRM protected; verify playback with the external disk connected.", true},
		{filepath.Join(home, "Library", "Containers", "com.apple.TV", "Data", "Library", "Downloads"), "Apple TV Downloads", "Apple TV app-managed download folder. Quit TV before moving. Downloads may be DRM protected; verify playback with the external disk connected.", true},
	}
	for _, candidate := range tvCandidates {
		if candidate.ifExisting {
			if _, err := os.Lstat(candidate.path); os.IsNotExist(err) {
				continue
			}
		}
		addAppleManagedCandidate(candidate.path, candidate.name, CategoryAppleMedia, "TV", candidate.note, associated, items, bySource)
	}
}

func appleMusicMediaFolder(home string) string {
	fallback := filepath.Join(home, "Music", "Music", "Media.localized")
	output, err := exec.Command("defaults", "read", "com.apple.Music", "media-folder-url").Output()
	if err != nil {
		return fallback
	}
	mediaURL, err := url.Parse(strings.TrimSpace(string(output)))
	if err != nil || mediaURL.Scheme != "file" || (mediaURL.Host != "" && mediaURL.Host != "localhost") || mediaURL.Path == "" {
		return fallback
	}
	return filepath.Clean(filepath.FromSlash(mediaURL.Path))
}

func appleTVMediaFolder(home string) string {
	fallback := filepath.Join(home, "Movies", "TV", "Media.localized")
	output, err := exec.Command("defaults", "read", "com.apple.TV", "media-folder-url").Output()
	if err != nil {
		return fallback
	}
	mediaURL, err := url.Parse(strings.TrimSpace(string(output)))
	if err != nil || mediaURL.Scheme != "file" || (mediaURL.Host != "" && mediaURL.Host != "localhost") || mediaURL.Path == "" {
		return fallback
	}
	return filepath.Clean(filepath.FromSlash(mediaURL.Path))
}
func addAppleManagedCandidate(path, name string, category Category, owner, note string, associated map[string]bool, items map[string]Item, bySource map[string]Operation) {
	if hasSymlinkParent(path) {
		return
	}
	if item, ok := makeItemAtLeast(path, name, category, owner, bySource, 0); ok {
		switch item.Status {
		case StatusMoved:
			item.Reason = "Already moved to external storage. Use Restore to bring it back."
		case StatusDiskOffline:
			item.Reason = "Already moved, but the destination disk is unavailable. Connect the disk to use or restore this data."
		case StatusUnavailable:
			if item.Reason != "" {
				item.Reason += " " + note
			} else {
				item.Reason = note
			}
		default:
			item.Reason = note
		}
		item.CreateWhenMissing = true
		if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink == 0 && !info.IsDir() {
			item.Status = StatusUnavailable
			item.Reason = "This Apple-managed location is not a directory. " + note
		}
		items[path] = item
		associated[path] = true
		return
	}
	_, statErr := os.Lstat(path)
	if os.IsNotExist(statErr) {
		items[path] = Item{ID: shortID(path), Name: name, Category: category, Status: StatusAvailable,
			SourcePath: path, Owner: owner, CreateWhenMissing: true,
			Reason: note + " The folder does not exist yet; moving it creates a link now so future data can be stored on the selected external disk."}
		associated[path] = true
		return
	}
	if os.IsPermission(statErr) {
		items[path] = Item{ID: shortID(path), Name: name, Category: category, Status: StatusUnavailable,
			SourcePath: path, Reason: "macOS denied access to this location. " + note}
		associated[path] = true
	}
}

func scanApplications(home string, items map[string]Item, bySource map[string]Operation) []appIdentity {
	roots := []string{"/Applications", filepath.Join(home, "Applications")}
	type appPath struct{ name, path string }
	type appResult struct {
		path string
		id   appIdentity
		item Item
		has  bool
	}
	var candidates []appPath
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".app") {
				continue
			}
			path := filepath.Join(root, name)
			if hasSymlinkParent(path) {
				continue
			}
			if !entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				continue
			}
			candidates = append(candidates, appPath{name: name, path: path})
		}
	}
	results := make(chan appResult, len(candidates))
	semaphore := make(chan struct{}, 4)
	var workers sync.WaitGroup
	for _, candidate := range candidates {
		workers.Add(1)
		go func(candidate appPath) {
			defer workers.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			identity := readAppIdentity(candidate.path)
			if identity.Name == "" {
				identity.Name = strings.TrimSuffix(candidate.name, filepath.Ext(candidate.name))
			}
			item, ok := makeItemAtLeast(candidate.path, identity.Name, CategoryApplication, "", bySource, candidateMinimumBytes)
			results <- appResult{path: candidate.path, id: identity, item: item, has: ok}
		}(candidate)
	}
	go func() {
		workers.Wait()
		close(results)
	}()
	collected := make([]appResult, 0, len(candidates))
	for result := range results {
		collected = append(collected, result)
	}
	sort.Slice(collected, func(i, j int) bool { return collected[i].path < collected[j].path })
	apps := make([]appIdentity, 0, len(collected))
	for _, result := range collected {
		apps = append(apps, result.id)
		if result.has {
			items[result.path] = result.item
		}
	}
	return apps
}

func scanAssociatedAppData(home string, apps []appIdentity, items map[string]Item, bySource map[string]Operation) map[string]bool {
	roots := []string{
		filepath.Join(home, "Library", "Application Support"),
		filepath.Join(home, "Library", "Containers"),
		filepath.Join(home, "Library", "Caches"),
		filepath.Join(home, "Library", "Logs"),
		filepath.Join(home, "Library", "Saved Application State"),
		filepath.Join(home, "Library", "Preferences"),
	}
	matched := make(map[string]bool)
	for _, root := range roots {
		if hasSymlinkAncestor(root) {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if path == root {
					return filepath.SkipDir
				}
				return nil
			}
			if path == root {
				return nil
			}
			if d.Type()&fs.ModeSymlink != 0 {
				rel, _ := filepath.Rel(root, path)
				if pathDepth(rel) <= 2 && isExtenderManagedLink(path) {
					owner := ownerForDataName(filepath.Base(path), apps)
					ownerName := ""
					if owner != nil {
						ownerName = owner.Name
					}
					if item, ok := makeItem(path, filepath.Base(path), CategoryAppData, ownerName, bySource); ok {
						item.Reason = "Previously moved by Extender; available to restore."
						items[path] = item
						matched[path] = true
					}
				}
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if pathDepth(rel) > 2 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			owner := ownerForDataName(filepath.Base(path), apps)
			if owner == nil {
				return nil
			}
			matched[path] = true
			if d.IsDir() {
				if item, ok := makeItemAtLeast(path, filepath.Base(path), CategoryAppData, owner.Name, bySource, candidateMinimumBytes); ok {
					items[path] = item
				}
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		})
	}
	return matched
}

func scanDeveloperData(home string, items map[string]Item, bySource map[string]Operation) {
	paths := []string{filepath.Join(home, "Library", "Developer")}
	paths = append(paths,
		filepath.Join(home, "Library", "Android", "sdk"),
		filepath.Join(home, ".android", "avd"),
	)
	studioRoots := []string{
		filepath.Join(home, "Library", "Application Support", "Google"),
		filepath.Join(home, "Library", "Caches", "Google"),
		filepath.Join(home, "Library", "Logs", "Google"),
	}
	for _, root := range studioRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			child := filepath.Join(root, entry.Name())
			if androidStudioName(entry.Name()) && (entry.IsDir() || (entry.Type()&fs.ModeSymlink != 0 && isExtenderManagedLink(child))) {
				paths = append(paths, child)
			}
		}
	}
	for _, path := range paths {
		if path == filepath.Join(home, "Library", "Developer") {
			if hasSymlinkAncestor(path) {
				continue
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				child := filepath.Join(path, entry.Name())
				if entry.Type()&fs.ModeSymlink != 0 && isExtenderManagedLink(child) {
					if item, ok := makeItem(child, entry.Name(), CategoryDeveloper, "", bySource); ok {
						item.Reason = "Previously moved by Extender; available to restore."
						items[child] = item
					}
				} else if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
					if item, ok := makeItemAtLeast(child, entry.Name(), CategoryDeveloper, "", bySource, candidateMinimumBytes); ok {
						items[child] = item
					}
				}
			}
			continue
		}
		if item, ok := makeItemAtLeast(path, filepath.Base(path), CategoryDeveloper, "", bySource, candidateMinimumBytes); ok {
			items[path] = item
		}
	}
}

func scanUnassignedData(home string, associated map[string]bool, items map[string]Item, bySource map[string]Operation) {
	roots := []string{
		filepath.Join(home, "Library", "Application Support"),
		filepath.Join(home, "Library", "Containers"),
		filepath.Join(home, "Library", "Caches"),
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil || hasSymlinkAncestor(root) {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(root, entry.Name())
			if entry.Type()&fs.ModeSymlink != 0 {
				if isExtenderManagedLink(path) {
					if item, ok := makeItem(path, entry.Name(), CategoryAppData, "", bySource); ok {
						item.Reason = "Previously moved by Extender; application owner could not be confirmed."
						items[path] = item
					}
				}
				continue
			}
			if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				continue
			}
			if filepath.Clean(root) == filepath.Clean(filepath.Join(home, "Library", "Containers")) && entry.Name() == "com.docker.docker" {
				continue
			}
			if filepath.Clean(root) == filepath.Clean(filepath.Join(home, "Library", "Application Support")) && entry.Name() == "rancher-desktop" {
				continue
			}
			if associated[path] {
				continue
			}
			if hasAssociatedDescendant(path, associated) {
				continue
			}
			_, large, err := allocatedSizeAtLeast(path, unassignedMinimumBytes)
			if err != nil || !large {
				continue
			}
			if item, ok := makeItem(path, entry.Name(), CategoryAppData, "", bySource); ok {
				item.Reason = "Large data folder; its app owner could not be confirmed. Review before moving."
				items[path] = item
			}
		}
	}
}

func scanContainerStorage(home string, items map[string]Item) {
	paths := []struct {
		path string
		name string
		note string
	}{
		{filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms"), "Docker Desktop storage", "Docker Desktop manages its VM disk. Move it in Docker Desktop Settings > Resources > Advanced; do not relocate the disk file directly."},
		{filepath.Join(home, "Library", "Group Containers", "HUAQ24HBR6.dev.orbstack", "data"), "OrbStack storage", "Use OrbStack's supported external-drive storage workflow. Directly moving its internal data folder is not enabled."},
		{filepath.Join(home, ".colima"), "Colima storage", "Colima supports a configurable home directory. This installation needs a provider-specific migration and environment configuration."},
		{filepath.Join(home, "Library", "Application Support", "rancher-desktop"), "Rancher Desktop storage", "Rancher Desktop uses VM-managed storage. Directly moving its internal data folder is not enabled."},
	}
	for _, candidate := range paths {
		if _, err := os.Lstat(candidate.path); err != nil {
			continue
		}
		item, ok := makeItem(candidate.path, candidate.name, CategoryContainer, "", nil)
		if !ok {
			continue
		}
		item.Status = StatusUnavailable
		item.Reason = candidate.note
		items[candidate.path] = item
	}
}

func makeItem(path, name string, category Category, owner string, bySource map[string]Operation) (Item, bool) {
	return makeItemAtLeast(path, name, category, owner, bySource, 0)
}

func makeItemAtLeast(path, name string, category Category, owner string, bySource map[string]Operation, minimum int64) (Item, bool) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Item{}, false
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Item{}, false
	}
	if !info.IsDir() && !info.Mode().IsRegular() && info.Mode()&fs.ModeSymlink == 0 {
		return Item{}, false
	}
	if minimum > 0 && info.Mode()&fs.ModeSymlink == 0 {
		if _, large, err := allocatedSizeAtLeast(path, minimum); err != nil || !large {
			return Item{}, false
		}
	}
	item := Item{ID: shortID(path), Name: name, Category: category, SourcePath: path,
		SizeBytes: 0, Status: StatusAvailable, Owner: owner}
	item.SourceFingerprint = fingerprint(info)
	measurePath := path
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			item.Status, item.Reason = StatusUnavailable, "Could not read the existing symbolic link."
			return item, true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		item.DestinationPath = filepath.Clean(target)
		if _, err := os.Stat(target); err != nil {
			item.Status, item.Reason = StatusDiskOffline, "The linked destination is currently unavailable."
		} else {
			item.Status = StatusMoved
			measurePath = target
		}
	}
	knownSize := false
	if op, ok := bySource[path]; ok {
		item.VolumeUUID = op.VolumeUUID
		if item.DestinationPath == "" {
			item.DestinationPath = op.DestinationPath
		}
		item.SizeBytes = op.SizeBytes
		knownSize = true
	}
	if !knownSize {
		if size, sockets, err := allocatedSizeWithRuntimeSockets(measurePath); err == nil {
			item.SizeBytes = size
			if category == CategoryAIData {
				item.RuntimeSockets = sockets
			}
		} else {
			item.Status = StatusUnavailable
			item.Reason = "Could not measure candidate contents: " + err.Error()
		}
	}
	return item, true
}

func allocatedSize(root string) (int64, error) {
	size, _, err := allocatedSizeWithRuntimeSockets(root)
	return size, err
}

func allocatedSizeWithRuntimeSockets(root string) (int64, int, error) {
	var total int64
	sockets := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if raw, ok := info.Sys().(*syscall.Stat_t); ok && raw.Blocks > 0 {
			total += raw.Blocks * 512
		} else if info.Mode().IsRegular() {
			total += info.Size()
		}
		if info.Mode()&fs.ModeSocket != 0 {
			sockets++
		}
		return nil
	})
	return total, sockets, err
}

func allocatedSizeAtLeast(root string, minimum int64) (int64, bool, error) {
	var total int64
	large := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if raw, ok := info.Sys().(*syscall.Stat_t); ok && raw.Blocks > 0 {
			total += raw.Blocks * 512
		} else if info.Mode().IsRegular() {
			total += info.Size()
		}
		if total >= minimum {
			large = true
			return fs.SkipAll
		}
		return nil
	})
	return total, large, err
}

func readAppIdentity(appPath string) appIdentity {
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	cmd := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", plist)
	data, err := cmd.Output()
	if err != nil {
		return appIdentity{}
	}
	var values map[string]any
	if json.Unmarshal(data, &values) != nil {
		return appIdentity{}
	}
	stringValue := func(key string) string {
		value, _ := values[key].(string)
		return value
	}
	name := stringValue("CFBundleDisplayName")
	if name == "" {
		name = stringValue("CFBundleName")
	}
	return appIdentity{Name: name, BundleID: stringValue("CFBundleIdentifier"), Executable: stringValue("CFBundleExecutable"), Path: appPath}
}

func ownerForDataName(name string, apps []appIdentity) *appIdentity {
	key := normalizeAppName(strings.TrimSuffix(name, filepath.Ext(name)))
	if key == "" || strings.HasPrefix(key, "group") {
		return nil
	}
	for i := range apps {
		if key == normalizeAppName(apps[i].Name) || (apps[i].BundleID != "" && key == normalizeAppName(apps[i].BundleID)) {
			return &apps[i]
		}
	}
	return nil
}

func normalizeAppName(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, value)
}

func androidStudioName(name string) bool {
	if !strings.HasPrefix(name, "AndroidStudio") {
		return false
	}
	rest := strings.TrimPrefix(name, "AndroidStudio")
	rest = strings.TrimPrefix(rest, "Preview")
	if rest == "" {
		return true
	}
	for _, r := range rest {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func hasSymlinkAncestor(path string) bool {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&fs.ModeSymlink != 0 {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func hasSymlinkParent(path string) bool {
	return hasSymlinkAncestor(filepath.Dir(filepath.Clean(path)))
}

func fingerprint(info os.FileInfo) string {
	if raw, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d:%d:%d:%d", raw.Dev, raw.Ino, raw.Mode, raw.Size, info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%d:%d:%s", info.Mode(), info.Size(), info.ModTime().UTC().Format(time.RFC3339Nano))
}

func hasAssociatedDescendant(parent string, associated map[string]bool) bool {
	for path := range associated {
		if strings.HasPrefix(path, parent+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func pathDepth(path string) int {
	if path == "." {
		return 0
	}
	return strings.Count(path, string(filepath.Separator)) + 1
}

func shortID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:6])
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		firstIsFutureApplePath := items[i].CreateWhenMissing && items[i].SizeBytes == 0
		secondIsFutureApplePath := items[j].CreateWhenMissing && items[j].SizeBytes == 0
		if firstIsFutureApplePath != secondIsFutureApplePath {
			return firstIsFutureApplePath
		}
		if items[i].Status != items[j].Status {
			return items[i].Status < items[j].Status
		}
		if items[i].SizeBytes != items[j].SizeBytes {
			return items[i].SizeBytes > items[j].SizeBytes
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

func debugItem(item Item) string {
	return fmt.Sprintf("%s (%s)", item.Name, item.ID)
}
