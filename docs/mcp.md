# CourseForge MCP

CourseForge ships an MCP server inside the main binary: `courseforge mcp`.
It lets an AI agent browse courses, read tasks, run solutions in the sandbox
and see your submissions.

Enable it first: **Settings → MCP-сервер** in the CourseForge UI. While it is
off, `courseforge mcp` exits with an error. The same page generates snippets
for your exact paths; the ones below assume `courseforge` is on `PATH`
(the release installers do that).

## Tools

| Tool | Purpose |
|------|---------|
| `list_courses` | courses, tracks, tasks and progress |
| `get_task_details` | statement, languages, limits |
| `get_task_template` | starter code |
| `get_task_tests` | unit tests |
| `get_task_solution` | reference solution |
| `run_solution` | run code against the tests |
| `list_submissions` | attempt history |
| `set_active_task_context` / `get_current_task` | the task currently in focus |

## Transports

- **stdio** (default): the agent starts `courseforge mcp` itself. The UI does
  not need to be running.
- **SSE**: the running app serves `http://127.0.0.1:8080/api/mcp/sse` (use your
  port). Standalone: `courseforge mcp --transport=sse --port=8085`, which
  serves `/sse` instead.

SSE works with Claude Code (`claude mcp add --transport sse courseforge <url>`),
Cursor (`"url"`), VS Code (`"type": "sse"`) and OpenCode (`"type": "remote"`).
Claude Desktop, Codex CLI, Windsurf and Antigravity CLI: use stdio.

## Clients (stdio)

**Claude Code**
```bash
claude mcp add courseforge -- courseforge mcp
```
Or install the plugin, which also adds a tutor skill:
```
/plugin marketplace add paintingpromisesss/courseforge
/plugin install courseforge@courseforge
```

**Claude Desktop** — `%APPDATA%\Claude\claude_desktop_config.json`
(macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`)
```json
{ "mcpServers": { "courseforge": { "command": "courseforge", "args": ["mcp"] } } }
```

**Cursor** — `~/.cursor/mcp.json` or `.cursor/mcp.json`, **Windsurf** —
`~/.codeium/windsurf/mcp_config.json`, **Antigravity CLI** —
`~/.gemini/config/mcp_config.json` (use the global file; the project-level
`.agents/mcp_config.json` may be ignored): same JSON as Claude Desktop.

**VS Code (Copilot)** — `.vscode/mcp.json`
```json
{ "servers": { "courseforge": { "type": "stdio", "command": "courseforge", "args": ["mcp"] } } }
```

**Codex CLI** — `~/.codex/config.toml`
```toml
[mcp_servers.courseforge]
command = "courseforge"
args = ["mcp"]
```

**OpenCode** — `opencode.json`
```json
{ "mcp": { "courseforge": { "type": "local", "command": ["courseforge", "mcp"] } } }
```

**Any other client**: run `courseforge mcp` over stdio.

Restart the client after editing its config.

## Plugins

The [`plugin/`](../plugin) folder bundles the MCP server and a tutor skill for
agents that have a plugin system. The same `mcp.json` and `skills/` are shared;
each agent has its own manifest next to them. Requires `courseforge` on `PATH`.

| Agent | Manifest | Install |
|-------|----------|---------|
| Claude Code | `.claude-plugin/` | `/plugin marketplace add paintingpromisesss/courseforge`, then `/plugin install courseforge@courseforge` |
| Codex CLI | `.codex-plugin/`, marketplace `.agents/plugins/` | `codex plugin marketplace add paintingpromisesss/courseforge` |
| Cursor | `.cursor-plugin/`, marketplace `.cursor-plugin/` | Dashboard → Plugins & MCPs → Add Marketplace → Import from Repo |

VS Code, Windsurf, OpenCode and Antigravity CLI have no plugin format for this:
use the MCP config above.

## Tutor instructions (any agent)

Paste into `AGENTS.md`, `CLAUDE.md`, Cursor rules or the system prompt:

```md
When I work on a CourseForge task:
1. Call `get_current_task` (or `list_courses` + `set_active_task_context`).
2. Read `get_task_details`, `get_task_tests` and `list_submissions`.
3. Hint in steps: idea, then approach, then a small snippet. One step per reply.
4. Verify my code with `run_solution` and explain failures from its output.
5. `get_task_solution` is for your own reference. Never show it unless I
   explicitly give up and ask for it.
```
