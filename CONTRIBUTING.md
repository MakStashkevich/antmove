# Contributing to antmove

Repository: https://github.com/MakStashkevich/antmove

Thanks for considering a contribution. Antmove handles real application data, so clear previews, recoverability, and conservative defaults matter more than adding a command quickly.

## Before changing code

1. Read [AGENTS.md](AGENTS.md) and the owning source files.
2. Check whether the behavior already exists in the TUI or CLI.
3. Keep changes narrow and preserve the transfer journal and rollback behavior.
4. For a new candidate path, explain how ownership is established, how active use is detected, and what happens when the volume disappears.
5. Update the seven localized READMEs and <code>llms.txt</code> when user-visible behavior changes.

## Requirements

- macOS 15 or later for native operation and release builds.
- Go 1.24 or later.
- Make is available with the Xcode Command Line Tools.

## Local development

~~~sh
make build
./bin/antmove --help
make test
make vet
make check
~~~

Useful release targets:

~~~sh
make build-arm64
make build-amd64
make release
~~~

<code>make release</code> writes architecture-specific archives under <code>dist/</code>. It does not publish a release or sign/notarize binaries.

The Homebrew formula is in <code>Formula/antmove.rb</code>. When preparing a release, update its version, pinned source archive URL, and SHA-256 together.

## Go style

- Run <code>gofmt</code> on every changed Go file.
- Prefer standard-library code and small focused functions.
- Quote paths and preserve Unicode and spaces.
- Return errors with enough context to identify the item and operation.
- Keep terminal output readable at narrow widths and retain English UI strings.
- Add or update tests for behavioral changes, including failure and recovery paths.
- Never use a real user library or disk as a test fixture. Use temporary directories and synthetic files.

## Transfer behavior

- Keep read-only scan and dry-run behavior free of writes.
- Never move active app or engine data without an explicit safe shutdown by the user.
- Preserve metadata with <code>ditto</code> and verify the staged copy before replacing the source.
- Keep operation state recoverable if any filesystem step fails.
- Do not directly move Docker or container engine VM/database stores.
- Treat Apple media, device backups, DRM, permissions, and missing external volumes explicitly.

## Pull requests

1. Create a focused branch from the project default branch.
2. Describe the user problem and the behavior change.
3. Include the commands used to build and verify the change.
4. Mention platform or permission limitations.
5. Do not claim a release; maintainers handle versioning and publication.

## License

By contributing, you agree that your contribution is distributed under the repository's MIT License. You retain your copyright; no separate CLA is required.

