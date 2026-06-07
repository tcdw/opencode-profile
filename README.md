# opencode-profile (`ocp`)

Isolated **profiles** for [opencode](https://opencode.ai) — switch API keys, system
prompt, skills, and MCP servers per profile, and launch opencode under the one
you pick.

Built because a single global `AGENTS.md` can't suit every model: a stylized
system prompt that's harmless to Claude/GPT may derail GLM/DeepSeek/Kimi/Qwen.
With `ocp` you keep a "Claude profile" with that prompt and a clean "domestic-model
profile" side by side, sharing the same API keys.

## How isolation works

opencode follows the XDG base-dir spec. `ocp` launches it with
`XDG_CONFIG_HOME` / `XDG_DATA_HOME` (and state/cache) pointed inside a profile,
so opencode resolves *all* of its config and data there — no files in your live
`~/.config/opencode` or `~/.local/share/opencode` are ever touched.

| Isolated thing | Lives in | Travels via |
|---|---|---|
| API keys | `data/opencode/auth.json`, `mcp-auth.json` | `XDG_DATA_HOME` |
| System prompt | `config/opencode/AGENTS.md` | `XDG_CONFIG_HOME` |
| Skills | `config/opencode/skills/` | `XDG_CONFIG_HOME` |
| MCP servers | `config/opencode/opencode.json` → `mcp` | `XDG_CONFIG_HOME` |

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
ocp path <name>                     # print export lines: eval "$(ocp path work)"
ocp init                            # create the store, seed shared from current config
```

The built-in **`default`** profile runs opencode against your live config (no override).

## Layout

```
~/.opencode-profiles/            # override with $OCP_HOME
  profiles.json                  # ocp metadata
  shared/{auth.json, mcp-auth.json, skills/}
  profiles/<name>/
    config/opencode/{opencode.json, AGENTS.md, skills/}
    data/opencode/{auth.json→shared, mcp-auth.json→shared, opencode.db, ...}
    state/  cache/
```

Profile `opencode.json` is edited surgically (via gjson/sjson), so toggling one
MCP server or changing the model leaves the rest of the file byte-for-byte intact.
