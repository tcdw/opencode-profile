# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-06-10

### Changed

- `ocp run` on unix now launches opencode as a child process instead of replacing the current process via `syscall.Exec`. Signals (SIGINT, SIGTERM, SIGQUIT, SIGHUP) are forwarded to the child, and stdio is inherited as before. This enables post-exit housekeeping that was impossible when ocp ceased to exist after launch.

### Added

- Post-exit credential sync: after opencode exits, any providers written to the XDG default data directory (`~/.local/share/opencode/auth.json`, `mcp-auth.json`) are merged back into the profile's auth files. This fixes the long-standing issue where `opencode auth login` and `/connect` would write credentials to the system-wide location instead of the profile, since opencode resolves its auth path from `XDG_DATA_HOME` rather than `OPENCODE_CONFIG_DIR`. Symlinked (linked-mode) profiles sync through to the shared base automatically.

## [0.4.0] - 2026-06-10

### Changed

- `ocp run` no longer overrides `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, or `XDG_CACHE_HOME` in the child environment. Only `OPENCODE_CONFIG_DIR`, `OPENCODE_CONFIG`, and `OPENCODE_DB` are set. This restores access to third-party tools (glab, gh, etc.) that rely on XDG directories for their own authentication tokens.

## [0.3.0] - 2026-06-08

### Added

- Support `opencode.jsonc` alongside `opencode.json` across profile creation, launch environment, TUI editors, and import/export bundles. Existing `.jsonc` files are preferred so a blank fallback `opencode.json` cannot mask the real config.
- Export/import validation for profile system prompts: bundles must include each profile's `AGENTS.md`, while empty prompts remain valid for profiles that intentionally do not use a custom system prompt.
- Creation-time hints when a profile inherits a non-empty live `AGENTS.md`, plus clearer TUI wording for the seed/blank choice so copied system prompts are visible instead of surprising.
- Windows-specific launch environment now exports explicit `OPENCODE_CONFIG_DIR`, `OPENCODE_CONFIG`, and `OPENCODE_DB` paths in addition to the XDG directories, matching opencode's config discovery more reliably on Windows.
- Tests covering JSONC round-trips, missing profile config, missing `AGENTS.md`, Windows symlink fallback behavior, and generated opencode environment variables.

### Changed

- Profile creation preserves the live config filename extension when seeding from the current opencode config.
- Import/export preserves `opencode.jsonc` entries instead of normalizing everything to `opencode.json`.
- The TUI and CLI now resolve the active profile config through the same config lookup helper used by launch.
- Windows symlink failures during mode changes fall back to owned copies, matching the existing import/materialization behavior.

### Fixed

- Custom providers and API-key settings were not initialized on Windows when the selected profile's config lived in `opencode.jsonc`.
- Export no longer silently skips profiles whose opencode config is missing.
- Import no longer creates profiles with an empty placeholder config or prompt when the bundle is missing required profile files.
- Release workflow compatibility was updated for newer GitHub Actions runtime behavior.

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

[Unreleased]: https://github.com/tcdw/opencode-profile/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/tcdw/opencode-profile/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/tcdw/opencode-profile/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tcdw/opencode-profile/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tcdw/opencode-profile/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/tcdw/opencode-profile/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/tcdw/opencode-profile/releases/tag/v0.1.0
