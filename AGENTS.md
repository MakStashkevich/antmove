# Antmove Agent Guide

This file is the repository contract for AI agents and contributors. Read it before changing code, documentation, release files, or transfer behavior.

## Product

Antmove is a macOS terminal utility for moving supported apps and data to external storage while leaving a symbolic link at the original path. Its purpose is storage relocation, not cleanup or deletion.

- Supported OS: macOS 15 or later.
- Implementation: Go; Bubble Tea is used for the terminal UI.
- Release architectures: Apple Silicon arm64 and Intel amd64.
- Runtime users do not need Go, Swift, or Xcode.
- The interactive interface is English. README documentation is maintained in English, Japanese, Simplified Chinese, Korean, French, Ukrainian, and Russian.
- Canonical repository: https://github.com/MakStashkevich/antmove. The Telegram community URL is still pending; do not invent it.

## Source map

| File | Responsibility |
| --- | --- |
| src/main.go | CLI routing, flags, help, command output |
| src/model.go, src/display.go | Core types, version metadata, and terminal output formatting |
| src/tui.go | Bubble Tea menu, navigation, confirmation and rendering |
| src/scanner.go | Candidate discovery, app association, sizing, status |
| src/volumes.go | External disk detection, suitability and speed measurement |
| src/transfer.go | Move/restore execution, verification, rollback and process checks |
| src/state.go | Operation journal and persistent antmove state |
| src/installer.go | First-run and explicit user installation of antmove/am |
| install.sh, install-local.sh | One-line release installer and installer bundled with local release archives |
| Formula/antmove.rb | Homebrew formula |
| src/uninstall.go | Two-confirmation removal of antmove and its state |
| src/extender_compat.go | Import and recovery of legacy Extender transfer records |
| src/platform.go, src/privilege.go | macOS-specific helpers and narrow privilege requests |
| src/core_test.go | Tests for core transfer and safety behavior |
| src/*.go | Go implementation and tests |
| build.sh, Makefile | Cross-architecture release archives and local developer targets |
| skills/antmove/SKILL.md | Canonical agent skill |
| plugins/antmove | Plugin packaging copy of the skill |

Keep a behavior change in its owning file. Avoid expanding the CLI surface when an existing menu or command can present the choice clearly.

## Safety contract

1. Scanning, status, disk listing, and dry-run are read-only. Prefer them before any operation that changes files.
2. A real move or restore requires the user's explicit request. Show or run a dry-run first, explain the paths, disk, size, and material warning, then wait for approval.
3. A caution-rated disk requires two separate confirmations. Never suggest that double confirmation makes an unsuitable disk recommended.
4. Never close an app unless the user separately confirms that Antmove may send it a graceful quit request. Let macOS request Automation permission; if denied, stop without moving files. For AI assistant data only, after the confirmed graceful quit and only once the main app has exited, Antmove may force-terminate helper processes verified to belong to that app bundle. Never force-quit the main app, discard unsaved work, or terminate Docker/container engines, backup, sync, or unrelated background services.
5. Copy with macOS ditto to a staging path, verify content, then create the original-path symlink. For AI data, verify Unix sockets have no open owners; temporarily move only inactive socket entries aside during copy, omit them from the destination as non-portable runtime endpoints, and restore them to the source on every failure. Never skip an active socket. Keep journal phases and recovery behavior consistent with the transfer state machine.
6. Do not remove the original data before verification. On failure, restore the source path or leave a clear recovery-required journal entry.
   Before deleting an original backup after linking, compare it with the destination again. If they differ or cleanup fails, preserve the backup and keep the operation in `cleanup_pending`; recovery must never ignore a cleanup error or mark the operation complete while the backup remains.
   Finder metadata files named `.DS_Store` and `._*` may be removed from the backup during cleanup; do not remove any other mismatched entry.
7. Preserve spaces, Unicode, and symlink semantics in paths. Quote every path passed to an external command.
8. Do not treat Docker Desktop, OrbStack, Colima, or Rancher Desktop engine stores as ordinary folders. Show supported vendor guidance instead.
9. Apple TV downloads may be DRM protected; Photos, Music, TV and device backups need app-specific warnings. Do not alter app preferences unless the feature explicitly supports that workflow.
10. Do not add background services, telemetry, remote Docker scanning, or undocumented private-storage discovery without an explicit product decision.

## Development workflow

- Format Go changes with gofmt.
- Run the focused Go tests for behavior changes and the full <code>make test</code> before a release.
- Run <code>make check</code> for formatting, vet, and tests.
- Interactive moves should report copied bytes, percentage, remaining bytes, and an approximate ETA while copying.
- Build both release architectures with <code>make release</code> or <code>./build.sh</code>.
- Keep the Homebrew formula version, source URL, and SHA-256 aligned with the release source commit.
- Pushing a matching <code>v*</code> tag publishes the release and automatically updates the Homebrew formula on <code>main</code> after the assets are published.
- Do not claim a release is published unless a GitHub release exists and its assets are verified.
- When a command, option, candidate type, or safety rule changes, update all seven README files, <code>llms.txt</code>, and the agent skill where applicable.

## CLI reference

Use <code>am --help</code> as the current source of truth. The supported commands are <code>am</code>, <code>am install</code>, <code>am uninstall</code>, <code>am scan</code>, <code>am disks</code>, <code>am move</code>, <code>am restore</code>, <code>am status</code>, <code>am recover</code>, and <code>am migrate</code>. Do not invent flags or claim that container engine stores are directly movable.

