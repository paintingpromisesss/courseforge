import { describe, it, expect } from 'vitest';
import { buildMCPClients } from './mcpClients';
import type { MCPStatusResponse } from '../api/types';

const base: MCPStatusResponse = {
  enabled: true,
  transport: 'stdio',
  host: '127.0.0.1',
  port: 8080,
  courses_dir: 'C:/cf/courses',
  data_dir: 'C:/cf/data',
  binary_path: 'C:/Program Files/cf/courseforge.exe',
  command: 'C:/Program Files/cf/courseforge.exe',
  args: ['mcp'],
  sse_url: 'http://127.0.0.1:8080/api/mcp/sse',
  platform: 'windows',
  tools_count: 9,
  available: true,
};

const get = (s: MCPStatusResponse, id: string) => buildMCPClients(s).find((c) => c.id === id)!;

describe('mcpClients', () => {
  it('stdio: quotes paths with spaces and uses each client schema', () => {
    expect(get(base, 'claude-code').snippet).toBe(
      'claude mcp add courseforge -- "C:/Program Files/cf/courseforge.exe" mcp',
    );
    expect(JSON.parse(get(base, 'cursor').snippet).mcpServers.courseforge.command).toBe(base.command);
    expect(JSON.parse(get(base, 'vscode').snippet).servers.courseforge.type).toBe('stdio');
    expect(JSON.parse(get(base, 'opencode').snippet).mcp.courseforge.command[0]).toBe(base.command);
    expect(get(base, 'codex').snippet).toContain('[mcp_servers.courseforge]');
    expect(JSON.parse(get(base, 'antigravity').snippet).mcpServers.courseforge.command).toBe(base.command);
  });

  it('sse: remote configs use url, stdio-only clients get no snippet', () => {
    const sse = { ...base, transport: 'sse' as const };
    expect(get(sse, 'claude-code').snippet).toContain('--transport sse');
    expect(get(sse, 'windsurf').snippet).toBe('');
    expect(JSON.parse(get(sse, 'vscode').snippet).servers.courseforge).toEqual({ type: 'sse', url: base.sse_url });
    expect(get(sse, 'claude-desktop').snippet).toBe('');
    expect(get(sse, 'codex').snippet).toBe('');
    expect(get(sse, 'antigravity').snippet).toBe('');
  });
});
