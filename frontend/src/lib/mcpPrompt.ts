import type { MCPStatusResponse } from '../api/types';

export function getClaudeConfigPath(platform: string): string {
  switch (platform) {
    case 'windows':
      return '%APPDATA%\\Claude\\claude_desktop_config.json';
    case 'darwin':
      return '~/Library/Application Support/Claude/claude_desktop_config.json';
    default:
      return '~/.config/Claude/claude_desktop_config.json';
  }
}

export function getAntigravityConfigPath(): string {
  return '~/.gemini/config/mcp_config.json';
}

export function getCursorConfigPath(): string {
  return '.cursor/mcp.json или Settings → Features → MCP';
}

export function buildMCPSetupPrompt(status: MCPStatusResponse): string {
  const platform = status.platform || 'windows';
  const coursesDir = status.courses_dir || './courses';
  const dataDir = status.data_dir || './data';
  const command = status.command || status.binary_path || (platform === 'windows' ? 'courseforge.exe' : 'courseforge');
  const args = status.args && status.args.length > 0 ? status.args : ['mcp', `--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`];
  const sseUrl = status.sse_url || `http://${status.host || '127.0.0.1'}:${status.port || 8080}/api/mcp/sse`;

  const lines: string[] = [];
  lines.push(
    '# Настройка CourseForge MCP для AI-ассистента',
    '',
    'Ты — AI-ассистент разработчика. Твоя задача — подключить MCP-сервер CourseForge к своей среде, чтобы иметь возможность исследовать курсы, читать условия задач, смотреть тесты и запускать код через CourseForge.',
    '',
  );

  if (!status.enabled) {
    lines.push(
      '⚠️ ВНИМАНИЕ: В интерфейсе CourseForge переключатель MCP сейчас ВЫКЛЮЧЕН.',
      'Для того чтобы подключение заработало, откройте CourseForge → Настройки → MCP-сервер и включите тумблер «Включить MCP-сервер».',
      '',
    );
  }

  lines.push(
    '## Параметры сервера CourseForge',
    `- Платформа хоста: ${platform}`,
    `- Статус в настройках: ${status.enabled ? 'Включен (активен)' : 'Отключен (требуется включить в настройках)'}`,
    `- Команда запуска (stdio): ${command} ${args.join(' ')}`,
    `- URL для SSE подключения: ${sseUrl}`,
    `- Директория курсов: ${coursesDir}`,
    `- Директория данных: ${dataDir}`,
    `- Доступно инструментов: ${status.tools_count || 9}`,
    '',
    '## Конфигурация для клиентов',
    '',
    `### Claude Desktop (${getClaudeConfigPath(platform)})`,
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            command,
            args,
          },
        },
      },
      null,
      2,
    ),
    '```',
    '',
    `### Cursor (${getCursorConfigPath()})`,
    '// Вариант 1 (SSE — без создания дополнительного процесса):',
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            url: sseUrl,
          },
        },
      },
      null,
      2,
    ),
    '```',
    '// Вариант 2 (stdio):',
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            command,
            args,
          },
        },
      },
      null,
      2,
    ),
    '```',
    '',
    `### Antigravity / Gemini CLI (${getAntigravityConfigPath()})`,
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            command,
            args,
          },
        },
      },
      null,
      2,
    ),
    '```',
    '',
    '## Инструкция для агента',
    '1. Убедись, что MCP-сервер включен в интерфейсе CourseForge (Настройки → MCP-сервер).',
    '2. Добавь секцию `"courseforge"` в конфигурационный файл MCP согласно выбранному клиенту.',
    '3. Перезапусти клиент или перезагрузи конфигурацию MCP.',
    '4. Вызови инструмент `list_courses` для проверки связи с платформой.',
    '5. При успехе подтверди готовность кратко: «MCP CourseForge успешно подключен. Доступно 9 инструментов.»',
  );

  return lines.join('\n');
}
