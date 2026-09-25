import type { MCPStatusResponse } from '../api/types';
import { DEFAULT_BACKEND_HOST, DEFAULT_BACKEND_PORT } from './constants';

export interface MCPClientGuide {
  id: string;
  name: string;
  /** Where the snippet goes (config file or "terminal"). */
  target: string;
  /** Extra note shown under the snippet; may be empty. */
  note: string;
  snippet: string;
}

const json = (v: unknown) => JSON.stringify(v, null, 2);
const shellQuote = (s: string) => (/\s/.test(s) ? `"${s}"` : s);

export function buildMCPClients(status: MCPStatusResponse): MCPClientGuide[] {
  const command = status.command || status.binary_path || 'courseforge';
  const args = status.args && status.args.length > 0 ? status.args : ['mcp'];
  const url = status.sse_url || `http://${status.host || DEFAULT_BACKEND_HOST}:${status.port || DEFAULT_BACKEND_PORT}/api/mcp/sse`;
  const sse = status.transport === 'sse';
  const cmdLine = [command, ...args].map(shellQuote).join(' ');
  const stdioOnly = 'Клиент поддерживает только stdio — переключите транспорт на stdio.';

  // Same shape for most clients: { mcpServers: { courseforge: { command, args } | { url } } }.
  const mcpServers = () => json({ mcpServers: { courseforge: sse ? { url } : { command, args } } });

  return [
    {
      id: 'claude-code',
      name: 'Claude Code',
      target: 'терминал',
      note: 'Или установите плагин CourseForge: /plugin marketplace add paintingpromisesss/courseforge',
      snippet: sse ? `claude mcp add --transport sse courseforge ${url}` : `claude mcp add courseforge -- ${cmdLine}`,
    },
    {
      id: 'claude-desktop',
      name: 'Claude Desktop',
      target: '%APPDATA%\\Claude\\claude_desktop_config.json (macOS: ~/Library/Application Support/Claude/claude_desktop_config.json)',
      note: sse ? stdioOnly : 'Добавьте в существующий mcpServers и перезапустите Claude Desktop.',
      snippet: sse ? '' : mcpServers(),
    },
    {
      id: 'cursor',
      name: 'Cursor',
      target: '~/.cursor/mcp.json или .cursor/mcp.json в проекте',
      note: '',
      snippet: mcpServers(),
    },
    {
      id: 'vscode',
      name: 'VS Code (Copilot)',
      target: '.vscode/mcp.json в проекте',
      note: '',
      snippet: json({ servers: { courseforge: sse ? { type: 'sse', url } : { type: 'stdio', command, args } } }),
    },
    {
      id: 'codex',
      name: 'Codex CLI',
      target: '~/.codex/config.toml',
      note: sse ? stdioOnly : 'Или командой: codex mcp add courseforge -- ' + cmdLine,
      snippet: sse
        ? ''
        : `[mcp_servers.courseforge]\ncommand = ${JSON.stringify(command)}\nargs = ${JSON.stringify(args)}`,
    },
    {
      id: 'antigravity',
      name: 'Antigravity CLI',
      target: '~/.gemini/config/mcp_config.json (глобальный; проектный .agents/mcp_config.json может игнорироваться)',
      note: sse ? stdioOnly : '',
      snippet: sse ? '' : mcpServers(),
    },
    {
      id: 'opencode',
      name: 'OpenCode',
      target: 'opencode.json в проекте',
      note: '',
      snippet: json({ mcp: { courseforge: sse ? { type: 'remote', url } : { type: 'local', command: [command, ...args] } } }),
    },
    {
      id: 'windsurf',
      name: 'Windsurf',
      target: '~/.codeium/windsurf/mcp_config.json',
      note: sse ? stdioOnly : '',
      snippet: sse ? '' : mcpServers(),
    },
    {
      id: 'generic',
      name: 'Другой',
      target: 'конфиг вашего клиента',
      note: 'Любой MCP-клиент: запустите команду по stdio' + (sse ? ` или подключитесь по SSE: ${url}` : '.'),
      snippet: cmdLine,
    },
  ];
}
