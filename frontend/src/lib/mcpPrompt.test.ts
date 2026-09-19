import { describe, it, expect } from 'vitest';
import { buildMCPSetupPrompt } from './mcpPrompt';
import type { MCPStatusResponse } from '../api/types';

describe('mcpPrompt', () => {
  it('generates setup prompt for stdio with OpenCode snippet, context and instructions', () => {
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
    expect(prompt).toContain('# Подключение MCP-сервера CourseForge');
    expect(prompt).toContain('## Контекст');
    expect(prompt).toContain('Бинарник уже собран:');
    expect(prompt).toContain('F:/Proga/courseforge/bin/courseforge.exe');
    expect(prompt).toContain('--courses-dir=F:/Proga/courseforge/courses');
    expect(prompt).toContain('--data-dir=F:/Proga/courseforge/data');
    expect(prompt).toContain('opencode.json');
    expect(prompt).toContain('"type": "local"');
    expect(prompt).toContain('.mcp.json');
    expect(prompt).toContain('Claude Code');
    expect(prompt).toContain('Cursor');
    expect(prompt).toContain('Cline / Roo Code');
    expect(prompt).toContain('Windsurf');
    expect(prompt).toContain('нужен ли перезапуск');
  });

  it('generates setup prompt for SSE transport', () => {
    const status: MCPStatusResponse = {
      enabled: true,
      transport: 'sse',
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
    expect(prompt).toContain('URL MCP-сервера:');
    expect(prompt).toContain('http://127.0.0.1:8080/api/mcp/sse');
    expect(prompt).toContain('OpenCode');
    expect(prompt).toContain('"type": "remote"');
  });

  it('includes warning when toggle is disabled', () => {
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
    expect(prompt).toContain('сервер сейчас выключен');
  });

  it('handles unavailable binary gracefully without claiming it is built', () => {
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
      available: false,
    };

    const prompt = buildMCPSetupPrompt(status);
    expect(prompt).not.toContain('Бинарник уже собран:');
    expect(prompt).toContain('Бинарник ещё не собран (требуется сборка или установка):');
    expect(prompt).toContain('.\\scripts\\build.ps1');
    expect(prompt).toContain('Бинарник CourseForge ещё не собран на диске');
  });
});
