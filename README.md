# opencode-profile (`ocp`)

Isolated **profiles** for [opencode](https://opencode.ai) — switch API keys, system
prompt, skills, and MCP servers per profile, and launch opencode under the one
you pick.

Built because a single global `AGENTS.md` can't suit every model: a stylized
system prompt that's harmless to Claude/GPT may derail GLM/DeepSeek/Kimi/Qwen.
With `ocp` you keep a "Claude profile" with that prompt and a clean "domestic-model
profile" side by side, sharing the same API keys.

## How isolation works

opencode supports explicit `OPENCODE_*` path overrides for config, database,
and related directories. `ocp` launches it with `OPENCODE_CONFIG_DIR`,
`OPENCODE_CONFIG`, and `OPENCODE_DB` pointed at the profile's directories while
leaving XDG variables unchanged — so tools like `glab` and `gh` keep finding
their own tokens in the standard `~/.config` paths.

| Isolated thing | Lives in | Via |
|---|---|---|
| API keys | `data/opencode/auth.json`, `mcp-auth.json` | `OPENCODE_CONFIG_DIR` |
| System prompt | `config/opencode/AGENTS.md` | `OPENCODE_CONFIG_DIR` |
| Skills | `config/opencode/skills/` | `OPENCODE_CONFIG_DIR` |
| MCP servers | `config/opencode/opencode.json[c]` → `mcp` | `OPENCODE_CONFIG` |
| Session DB | `data/opencode/opencode.db` | `OPENCODE_DB` |

### Shared base + per-domain override

A `shared/` store holds `auth.json`, `mcp-auth.json`, and `skills/`. By default
each profile **symlinks** these from the base (so you don't re-login per profile),
while the system prompt, model, MCP config, and session DB stay per-profile. Any
domain can be flipped to **owned** (an isolated copy) — and back, with the old
copy backed up, never deleted.

## Build

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
ocp list                            # list profiles
ocp create <name> [-desc ..] [-blank]
ocp rm <name>
ocp export [names...] [-o b.zip]    # encrypted, portable bundle (all profiles if none named)
ocp import <bundle.zip> [--force]   # restore into the current store
ocp path <name>                     # print export lines: eval "$(ocp path work)"
ocp init                            # create the store, seed shared from current config
```

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
