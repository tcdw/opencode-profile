[English](README.md) | [简体中文](README.zh-CN.md)

# opencode-profile (`ocp`)

Isolated **profiles** for [opencode](https://opencode.ai) — switch API keys, system
prompt, skills, and MCP servers per profile, and launch opencode under the one
you pick.

## Why

**Model-dependent system prompts.** A stylized `AGENTS.md` that's fine with
Claude or GPT can derail GLM, DeepSeek, Kimi, or Qwen. Profiles let you keep a
"Claude profile" with that prompt and a clean "domestic-model profile" side by
side.

**Provider / Gateway isolation.** Accidentally routing personal work through
your company's AI Gateway is a mistake you want to make exactly once. Put the
internal gateway — with per-agent model overrides like
[#6019](https://github.com/anomalyco/opencode/issues/6019) — in one profile and
your personal API keys in another. No config edits when switching contexts, no
cross-contamination.

## How isolation works

opencode supports explicit `OPENCODE_*` path overrides for config and database
locations. `ocp` launches it with `OPENCODE_CONFIG_DIR`, `OPENCODE_CONFIG`, and
`OPENCODE_DB` pointed at the profile's directories while leaving XDG variables
unchanged — so tools like `glab` and `gh` keep finding their own tokens in the
standard `~/.config` paths. Since opencode still resolves credential files from
the XDG default data directory, `ocp` syncs the profile's auth files into that
location before launch and merges changes back after exit.

| Isolated thing | Lives in | Via |
|---|---|---|
| API keys | `data/opencode/auth.json`, `mcp-auth.json` | startup/exit sync |
| System prompt | `config/opencode/AGENTS.md` | `OPENCODE_CONFIG_DIR` |
| Skills | `config/opencode/skills/` | `OPENCODE_CONFIG_DIR` |
| MCP servers | `config/opencode/opencode.json[c]` → `mcp` | `OPENCODE_CONFIG` |
| Session DB | `data/opencode/opencode.db` | `OPENCODE_DB` |

### Isolation caveats

OpenCode merges config files with a **shallow merge**, not a full replacement. This means any object-type key (`mcp`, `provider`, `agent`, `command`, `permission`, `tools`, etc.) defined in your global `~/.config/opencode/opencode.json` will **leak into every profile** and get merged with whatever the profile itself declares.

This is the same for the `agents/`, `commands/`, `skills/`, and `plugins/` directories under `~/.config/opencode/` — they are combined with the profile-specific directories.

The safest way to avoid this is to **treat `~/.config/opencode/` as unmanaged** after you start using `ocp`:

1. Run `ocp init` to seed the shared store from your existing global config.
2. Create profiles from that shared base (`ocp create <name>`).
3. **Clear or delete** `~/.config/opencode/opencode.json` (and any `tui.json`) so there is no global fallback to merge with.
4. Keep only truly machine-wide settings in the global config (e.g., shell path) that you want every profile to inherit.

> If you must keep a global `opencode.json`, be aware that every object key inside it will silently merge with all your profiles. You can inspect the effective config at any time with `opencode debug config`.

### Shared base + per-domain override

A `shared/` store holds `auth.json`, `mcp-auth.json`, and `skills/`. By default
each profile **symlinks** these from the base (so you don't re-login per profile),
while the system prompt, model, MCP config, and session DB stay per-profile. Any
domain can be flipped to **owned** (an isolated copy) — and back, with the old
copy backed up, never deleted.

## Installation

### go install

```sh
go install github.com/tcdw/opencode-profile@latest
```

### GitHub Releases

Download pre-built binaries from the [Releases](https://github.com/tcdw/opencode-profile/releases) page. Archives are available for macOS, Linux, and Windows.

### Build from source

```sh
go build -o ocp .
# optional: put it on PATH
install ocp ~/.local/bin/ocp
```

## Usage

Run with no arguments for the interactive picker:

```
ocp
```

- `enter` launch the selected profile (replaces `ocp` with opencode)
- `n` new · `e` edit · `d` delete · `/` filter · `q` quit
- in **edit**: system prompt (`$EDITOR`), model, MCP toggles, providers, domain link/own

CLI:

```sh
ocp run <name> [-- opencode args]   # launch directly (good for shell aliases)
ocp acp <name> [-- opencode args]   # launch OpenCode ACP under a profile
ocp list                            # list profiles
ocp create <name> [-desc ..] [-blank]
ocp rm <name>
ocp export [names...] [-o b.zip]    # encrypted, portable bundle (all profiles if none named)
ocp import <bundle.zip> [--force]   # restore into the current store
ocp path <name>                     # print export lines: eval "$(ocp path work)"
ocp zed [names...]                  # print a Zed agent_servers snippet for ACP
ocp init                            # create the store, seed shared from current config
```

## Zed / ACP

OpenCode's ACP setup normally asks Zed to run `opencode acp` directly. With
profiles, point Zed at `ocp` instead so the profile environment is applied before
OpenCode starts.

Generate a ready-to-paste snippet for every profile:

```sh
ocp zed
```

Or generate entries for specific profiles:

```sh
ocp zed work personal
```

Add the printed JSON under `agent_servers` in `~/.config/zed/settings.json`. It
will look like this:

```json
{
  "agent_servers": {
    "OpenCode (work)": {
      "type": "custom",
      "command": "/absolute/path/to/ocp",
      "args": ["acp", "work"]
    }
  }
}
```

Create one entry per profile, then choose the matching OpenCode agent from
Zed's agent panel. Use the generated absolute `command` path because GUI apps may
not inherit your shell `PATH`.

## Moving profiles between machines

`ocp export` writes a single self-contained `.zip` you can carry anywhere (e.g.
to a Windows box). The bundle is portable by design:

- **Config travels in plaintext** — `opencode.json`/`opencode.jsonc`, `AGENTS.md`, and skills are
  readable/diffable inside the zip.
- **Secrets are encrypted** — `auth.json`, `mcp-auth.json`, and any `*.key`
  are packed into one `secrets.enc` blob (AES-256-GCM, key derived from your
  passphrase via PBKDF2). You're prompted for a passphrase, or set
  `OCP_PASSPHRASE` for non-interactive use.
- **No symlinks, no machine-specific paths** — link/own state is recorded as
  metadata and rebuilt on import (a `linked` domain that can't be symlinked,
  e.g. on Windows without privilege, degrades to an owned copy). Absolute
  `{file:...}` references in opencode config are rewritten to the new machine's
  store root. The 246 MB session DB, caches, and `.bak` files are never included.

```sh
ocp export -o work.zip                 # bundle every profile
ocp export rc-intl rc-cn -o rc.zip     # just these two
ocp import work.zip                     # restore (skips names that already exist)
ocp import work.zip --force             # overwrite existing profiles + shared secrets
```

After importing, re-run any login that wasn't carried (`ocp run <name> -- auth login`),
or just rely on the bundled secrets if you exported them.

The built-in **`default`** profile runs opencode against your live config (no override).

## Layout

```
~/.opencode-profiles/            # override with $OCP_HOME
  profiles.json                  # ocp metadata
  shared/{auth.json, mcp-auth.json, skills/}
  profiles/<name>/
    config/opencode/{opencode.json[c], AGENTS.md, skills/}
    data/opencode/{auth.json→shared, mcp-auth.json→shared, opencode.db, ...}
    state/  cache/
```

Profile opencode config is edited surgically (via gjson/sjson), so toggling one
MCP server or changing the model leaves the rest of the file byte-for-byte intact.
