[English](README.md) | [简体中文](README.zh-CN.md)

# opencode-profile（`ocp`）

为 [opencode](https://opencode.ai) 提供隔离的 **profiles** — 按 profile 切换 API key、系统提示词、skills 和 MCP 服务器，并在你选定的 profile 下启动 opencode。

## 为什么需要它

**模型依赖的系统提示词。** 一份带有风格化 `AGENTS.md` 的提示词对 Claude 或 GPT 可能没问题，但到了 GLM、DeepSeek、Kimi 或 Qwen 上就可能翻车。Profiles 让你可以并排保留一份带该提示词的「Claude profile」和一份干净的「国产模型 profile」。

**供应商 / 网关隔离。** 不小心把私人工作流量走公司 AI Gateway，这种错误犯一次就够了。把内部网关（及其按 agent 的模型覆盖，参考 [#6019](https://github.com/anomalyco/opencode/issues/6019)）放在一个 profile，把个人 API key 放在另一个。切换上下文无需改配置，不会交叉污染。

## 隔离机制

opencode 支持通过 `OPENCODE_*` 环境变量显式覆盖 config、database 等目录。`ocp` 启动时会将 `OPENCODE_CONFIG_DIR`、`OPENCODE_CONFIG` 和 `OPENCODE_DB` 指向 profile 专属目录，同时保持 XDG 变量不变 —— 这样 `glab` 和 `gh` 等工具仍然能在标准的 `~/.config` 路径下找到它们自己的 token。由于 opencode 仍然从 XDG 默认数据目录解析凭据文件，`ocp` 会在启动前把 profile 的 auth 文件同步过去，并在退出后把变更合并回 profile。

| 隔离项 | 所在位置 | 方式 |
|---|---|---|
| API keys | `data/opencode/auth.json`、`mcp-auth.json` | 启动/退出同步 |
| 系统提示词 | `config/opencode/AGENTS.md` | `OPENCODE_CONFIG_DIR` |
| Skills | `config/opencode/skills/` | `OPENCODE_CONFIG_DIR` |
| MCP 服务器 | `config/opencode/opencode.json[c]` → `mcp` | `OPENCODE_CONFIG` |
| 会话数据库 | `data/opencode/opencode.db` | `OPENCODE_DB` |

### 隔离注意事项

OpenCode 对配置文件采用**浅合并（shallow merge）**，而非完全替换。这意味着你在全局 `~/.config/opencode/opencode.json` 中定义的任何 object 类型键（`mcp`、`provider`、`agent`、`command`、`permission`、`tools` 等）都会**泄漏到每个 profile 中**，并与 profile 自身声明的配置合并。

`~/.config/opencode/` 下的 `agents/`、`commands/`、`skills/`、`plugins/` 目录同理 —— 它们也会与 profile 专属目录合并。

最安全的做法是：开始使用 `ocp` 后，**将 `~/.config/opencode/` 视为不再手动管理**：

1. 运行 `ocp init` 从现有全局配置中播种共享存储。
2. 从共享基础创建 profile（`ocp create <name>`）。
3. **清空或删除** `~/.config/opencode/opencode.json`（以及任何 `tui.json`），这样就没了会被合并的全局后备配置。
4. 仅在全局配置中保留真正全机统一的设置（如 shell 路径），即你希望每个 profile 都继承的内容。

> 如果必须保留全局的 `opencode.json`，请注意其中的每个 object 键都会静默合并到所有 profile 中。你可以随时通过 `opencode debug config` 查看实际生效的配置。

### 共享基础 + 按域覆盖

`shared/` 存储中保存着 `auth.json`、`mcp-auth.json` 和 `skills/`。默认每个 profile **符号链接**到这些基础文件（这样无需每个 profile 都重新登录），而系统提示词、模型、MCP 配置和会话数据库则按 profile 独立。任何域都可以被切换为 **owned**（独立副本）—— 也可以再切回去，旧副本会被备份，永不删除。

## 安装

### go install

```sh
go install github.com/tcdw/opencode-profile@latest
```

### GitHub Releases

从 [Releases](https://github.com/tcdw/opencode-profile/releases) 页面下载预编译二进制文件。提供 macOS、Linux 和 Windows 的归档包。

### 从源码构建

```sh
go build -o ocp .
# 可选：放入 PATH
install ocp ~/.local/bin/ocp
```

## 用法

不带参数运行进入交互式选择器：

```
ocp
```

- `enter` 启动选中的 profile（用 opencode 替换 `ocp` 进程）
- `n` 新建 · `e` 编辑 · `d` 删除 · `/` 过滤 · `q` 退出
- **编辑模式**中：系统提示词（`$EDITOR`）、模型、MCP 开关、provider、域的 link/own

CLI 命令：

```sh
ocp run <name> [-- opencode args]   # 直接启动（适用于 shell alias）
ocp acp <name> [-- opencode args]   # 在 profile 下启动 OpenCode ACP
ocp list                            # 列出所有 profile
ocp create <name> [-desc ..] [-blank]
ocp rm <name>
ocp export [names...] [-o b.zip]    # 加密便携打包（不指定名称则导出全部）
ocp import <bundle.zip> [--force]   # 恢复到当前存储
ocp path <name>                     # 打印 export 行：eval "$(ocp path work)"
ocp zed [names...]                  # 打印 Zed ACP 的 agent_servers 片段
ocp init                            # 创建存储，从当前配置播种共享数据
```

## Zed / ACP

OpenCode 的 ACP 文档通常让 Zed 直接运行 `opencode acp`。使用 profiles 时，
应让 Zed 运行 `ocp`，这样 OpenCode 启动前就会带上对应 profile 的环境变量。

为所有 profile 生成可直接粘贴的配置片段：

```sh
ocp zed
```

也可以只生成指定 profile 的条目：

```sh
ocp zed work personal
```

把输出的 JSON 合并到 `~/.config/zed/settings.json` 的 `agent_servers` 中。格式类似：

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

每个 profile 可以生成一个条目，然后在 Zed 的 agent panel 中选择对应的
OpenCode agent。建议使用生成出的绝对 `command` 路径，因为 GUI 应用不一定继承
你的 shell `PATH`。

## 跨设备迁移 profile

`ocp export` 会生成一个自包含的 `.zip`，可以随身携带到任何地方（比如带到 Windows 机器上）。打包方式在设计上就是可移植的：

- **配置以明文传输** — `opencode.json`/`opencode.jsonc`、`AGENTS.md`、skills、实时全局 opencode 配置（`~/.config/opencode/`），以及 `extensions/` 这类可迁移的全局数据在 zip 中可读、可 diff。
- **密钥加密存储** — `auth.json`、`mcp-auth.json` 以及所有 `*.key` 被打包到单一的 `secrets.enc` 二进制 blob 中（AES-256-GCM，密钥通过 PBKDF2 从你的密码导出）。会提示输入密码，也可设置 `OCP_PASSPHRASE` 用于非交互场景。
- **无符号链接，无机器特定路径** — link/own 状态以元数据形式记录，导入时重建（无法创建符号链接的 `linked` 域，如在无权限的 Windows 上，会降级为 owned 副本）。opencode 配置中的绝对路径 `{file:...}` 引用会被重写为指向新机器的存储根目录。会话/runtime 数据（`opencode.db*`、`snapshot/`、`storage/`、`tool-output/`、`log/`、`bin/`）、依赖树、`.bak` 文件，以及已由共享层管理的条目（`skills/`、`auth.json`、`mcp-auth.json`）不会被重复打包到 global 部分。

```sh
ocp export -o work.zip                 # 打包所有 profile
ocp export rc-intl rc-cn -o rc.zip     # 仅导出这两个
ocp import work.zip                     # 恢复（已存在的名称会跳过）
ocp import work.zip --force             # 覆盖已有 profile 和共享密钥
```

导入完成后，重新运行任何未被携带的登录操作（`ocp run <name> -- auth login`），或者如果已导出密钥的话，直接使用即可。

内置的 **`default`** profile 直接以你的实时配置运行 opencode（不做任何覆盖）。

## 目录结构

```
~/.opencode-profiles/            # 可通过 $OCP_HOME 覆盖
  profiles.json                  # ocp 元数据
  shared/{auth.json, mcp-auth.json, skills/}
  global/
    config/opencode → ~/.config/opencode        # 软链接，方便编辑
    data/opencode   → ~/.local/share/opencode   # 软链接，方便编辑
  profiles/<name>/
    config/opencode/{opencode.json[c], AGENTS.md, skills/}
    data/opencode/{auth.json→shared, mcp-auth.json→shared, opencode.db, ...}
    state/  cache/
```

Profile 的 opencode 配置以精确方式编辑（通过 gjson/sjson），因此切换一个 MCP 服务器或更改模型时，文件的其余部分保持逐字节不变。
