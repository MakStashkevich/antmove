---
name: antmove
description: "Use the antmove macOS CLI to inspect external disks and safely move or restore supported apps and data. Always read the scan and disk assessment first; preview each transfer and get explicit approval before changing files."
when_to_use: "mac disk full, move Mac apps to external drive, move application data, relocate Photos library, Apple Music media, Apple TV downloads, iPhone backup, Android SDK, Xcode data, restore antmove transfer, Extender migration, antmove, am move, external storage"
---

# antmove agent skill

Antmove moves supported apps and data to external disks and leaves a symbolic link at the original path. Transfers change real user files. Use the CLI as a careful operator: inspect first, show the plan, and let the user decide.

## Identify the installed version

Run <code>am --version</code>. If <code>am</code> is unavailable, explain that the CLI is not installed and offer the installation paths in the README. Do not install unless the user asked for installation.

Use <code>am --help</code> as the source of truth for the installed version. Never invent flags.

## Read-only first

For storage questions, start with:

- <code>am scan</code> to list candidates, statuses, sizes, and notes.
- <code>am disks</code> to see destination formats, free space, and suitability.
- <code>am status</code> to see recorded operations and whether their disks are connected.

Do not infer ownership from a folder name alone. Items marked unassigned need the user's review. Docker Desktop, OrbStack, Colima, and Rancher Desktop engine stores are guidance-only and must be moved using the engine's supported settings.

## Before moving

1. Select only the item the user requested.
2. Confirm the source and exact destination disk from Antmove's output.
3. Preview with <code>am move --disk PATH --dry-run ID</code>.
4. Explain the paths, size, available disk space, format/suitability, warnings, whether an app must be closed, and that the external disk must stay connected.
5. Wait for an explicit approval of that plan before running a real move.
6. For a caution-rated disk, antmove requires two separate confirmations. Do not bypass or emulate these confirmations.

Never close the main app unless the user separately confirms a graceful quit request. For AI assistant data only, after that confirmed quit and once the app has exited, Antmove may force-terminate helper processes verified to belong to the same app bundle. Never terminate a CLI in a terminal, backup/sync, container engine, or unrelated service.

Interactive moves show copied bytes, percentage, bytes remaining, and an approximate ETA. Before deleting an original backup after linking, compare its remaining data with the destination. `.DS_Store` and `._*` Finder metadata files may be removed from the backup; preserve any other mismatched data and keep the operation in `cleanup_pending`. Recovery must not ignore cleanup errors or mark the operation complete while a backup remains.

A real non-interactive transfer to a recommended disk uses <code>am move --disk PATH --yes ID</code>. Only run it after approval. Do not combine a caution disk with an automatic confirmation.

## Apple data

- `am scan` also lists recognized AI assistant data folders in common user-data locations, including `.codex`, `.agents`, `.claude`, `.copilot`, `.gemini`, `.qwen`, `.antigravity`, `.kilocode`, `.hermes`, `.config/opencode`, and known app-support folders. These may contain conversation history and project context. Runtime Unix sockets are non-portable: verify that none are active, temporarily set inactive sockets aside for copying, omit them from the destination, and restore them to the source if a transfer fails. Never skip an active socket. Ask the user to close the assistant and finish sync/generation before moving; review the folder first.

- Photos: ask the user to quit Photos; libraries should use APFS or Mac OS Extended. After moving, the user may need to open the library from the new location and set it as the System Photo Library.
- Music: quit Music first. Antmove does not edit Music preferences; the user should review its Media Location setting.
- Apple TV: quit TV and verify downloaded titles play from the external disk. Downloads can be DRM protected.
- iPhone/iPad backups: wait for backup and sync to finish. If access is denied, explain that the terminal app may need Full Disk Access; do not attempt to grant it automatically.
- For any external library, remind the user that the disk must be mounted when the data is used.

## Restore and recovery

Preview a restore with <code>am restore --dry-run ID</code>, explain what will be copied back, and get approval. Then use <code>am restore --yes ID</code> only for the approved items.

For interrupted operations, read <code>am status</code> first. <code>am recover</code> changes files; run it only if the user asked to recover or approved the described recovery.

## Installation and removal

Installation does not scan or move data. Offer either Homebrew (<code>brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git</code>, then <code>brew install antmove</code>) or the one-line Bash release installer in the README. Explain what installation changes and verify with <code>am --version</code>.

<code>am uninstall</code> removes antmove and its local state after two confirmations. It does not restore moved items. Run it only when the user explicitly asks to uninstall, after explaining the effect and checking whether anything still relies on external links.

## Reporting

Use Antmove's reported sizes and statuses. After a real operation, report what moved or restored, where it is stored, and any failure or unavailable disk. Never claim that a transfer succeeded based only on a command being started.

