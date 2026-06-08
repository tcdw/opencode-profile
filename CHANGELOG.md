# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-06-07

### Added

- `ocp export` and `ocp import`: move profiles between machines through a single portable, encrypted `.zip` bundle — the groundwork for cross-platform (Windows) use. Config (`opencode.json`, `AGENTS.md`, skills) travels in plaintext, while secrets (`auth.json`, `mcp-auth.json`, `*.key`) are packed into one AES-256-GCM `secrets.enc` blob whose key is derived from a passphrase via PBKDF2. Supply the passphrase interactively or with `OCP_PASSPHRASE`.
- Windows launch support: opencode is started as a child process (stdio and environment forwarded, exit code mirrored), since unix-style `exec()` process replacement is not available on Windows.

### Changed

- The launch handoff is now platform-split: `syscall.Exec` on unix and child-process execution on Windows (`internal/launch/exec_{unix,windows}.go`).
- A `linked` domain automatically degrades to an owned copy when the filesystem refuses symlinks (e.g. Windows without the symlink privilege), so import never leaves a profile half-built.

### Security

- Bundles never store secrets in their plaintext region. Export warns when `opencode.json` holds a literal API key instead of a `{file:}`/`{env:}` reference, and import guards against path traversal (zip-slip) while rewriting absolute `{file:}` paths to the target machine's store root.

## [0.1.1] - 2026-06-07

### Fixed

- Windows build: store file locking now compiles and works on Windows via `LockFileEx`/`UnlockFileEx` (`internal/store/lock_windows.go`), complementing the unix `flock` path.

## [0.1.0] - 2026-06-07

### Added

- Initial release of `opencode-profile` (`ocp`): isolated opencode profiles for API keys, system prompt (`AGENTS.md`), skills, and MCP servers, launched by redirecting `XDG_CONFIG_HOME`/`XDG_DATA_HOME` per profile.
- Shared base with per-domain override (linked symlink vs. owned copy), an interactive TUI picker and a CLI (`run`/`list`/`create`/`rm`/`path`/`init`), surgical `opencode.json` edits via gjson/sjson, and a built-in `default` profile that runs against the live config.
- GoReleaser configuration and a tag-triggered release workflow.

[Unreleased]: https://github.com/tcdw/opencode-profile/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/tcdw/opencode-profile/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/tcdw/opencode-profile/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/tcdw/opencode-profile/releases/tag/v0.1.0
