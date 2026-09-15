import { describe, it, expect } from 'vitest';
import { buildMCPSetupPrompt, getClaudeConfigPath } from './mcpPrompt';
import type { MCPStatusResponse } from '../api/types';

describe('mcpPrompt', () => {
  it('returns appropriate config path for windows', () => {
    expect(getClaudeConfigPath('windows')).toContain('%APPDATA%\\Claude');
  });

  it('returns appropriate config path for macos', () => {
    expect(getClaudeConfigPath('darwin')).toContain('Application Support/Claude');
  });

  it('generates complete prompt with available binary', () => {
    const status: MCPStatusResponse = {
      enabled: true,
      transport: 'stdio',
      host: '127.0.0.1',
      port: 8090,
      courses_dir: 'F:/Proga/courseforge/courses',
      data_dir: 'F:/Proga/courseforge/data',
      binary_path: 'F:/Proga/courseforge/bin/courseforge-mcp.exe',
      platform: 'windows',
      tools_count: 9,
      available: true,
    };

    const prompt = buildMCPSetupPrompt(status);
    expect(prompt).toContain('Настройка CourseForge MCP для AI-ассистента');
    expect(prompt).toContain('F:/Proga/courseforge/bin/courseforge-mcp.exe');
    expect(prompt).toContain('--courses-dir=F:/Proga/courseforge/courses');
    expect(prompt).toContain('--data-dir=F:/Proga/courseforge/data');
    expect(prompt).toContain('list_courses');
    expect(prompt).not.toContain('Шаг 1: Сборка MCP-бинарника');
  });

  it('includes build instructions when binary is not available', () => {
    const status: MCPStatusResponse = {
      enabled: true,
      transport: 'stdio',
      host: '127.0.0.1',
      port: 8090,
      courses_dir: './courses',
      data_dir: './data',
      binary_path: '',
      platform: 'linux',
      tools_count: 9,
      available: false,
    };

    const prompt = buildMCPSetupPrompt(status);
    expect(prompt).toContain('Шаг 1: Сборка MCP-бинарника');
    expect(prompt).toContain('go build -o ./bin/courseforge-mcp ./cmd/mcp');
  });
});
