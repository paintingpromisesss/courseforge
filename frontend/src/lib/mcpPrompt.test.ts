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

  it('generates complete prompt with main binary command and args', () => {
    const status: MCPStatusResponse = {
      enabled: true,
      transport: 'stdio',
      host: '127.0.0.1',
      port: 8090,
      courses_dir: 'F:/Proga/courseforge/courses',
      data_dir: 'F:/Proga/courseforge/data',
      binary_path: 'F:/Proga/courseforge/bin/courseforge.exe',
      command: 'F:/Proga/courseforge/bin/courseforge.exe',
      args: ['mcp', '--courses-dir=F:/Proga/courseforge/courses', '--data-dir=F:/Proga/courseforge/data'],
      sse_url: 'http://127.0.0.1:8080/api/mcp/sse',
      platform: 'windows',
      tools_count: 9,
      available: true,
    };

    const prompt = buildMCPSetupPrompt(status);
    expect(prompt).toContain('Настройка CourseForge MCP для AI-ассистента');
    expect(prompt).toContain('F:/Proga/courseforge/bin/courseforge.exe');
    expect(prompt).toContain('--courses-dir=F:/Proga/courseforge/courses');
    expect(prompt).toContain('--data-dir=F:/Proga/courseforge/data');
    expect(prompt).toContain('http://127.0.0.1:8080/api/mcp/sse');
    expect(prompt).toContain('list_courses');
    expect(prompt).not.toContain('переключатель MCP сейчас ВЫКЛЮЧЕН');
  });

  it('warns when toggle is disabled', () => {
    const status: MCPStatusResponse = {
      enabled: false,
      transport: 'stdio',
      host: '127.0.0.1',
      port: 8090,
      courses_dir: './courses',
      data_dir: './data',
      binary_path: 'courseforge',
      command: 'courseforge',
      args: ['mcp', '--courses-dir=./courses', '--data-dir=./data'],
      platform: 'linux',
      tools_count: 9,
      available: true,
    };

    const prompt = buildMCPSetupPrompt(status);
    expect(prompt).toContain('переключатель MCP сейчас ВЫКЛЮЧЕН');
    expect(prompt).toContain('Отключен (требуется включить в настройках)');
  });
});
