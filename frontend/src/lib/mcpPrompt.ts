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
  const binary = status.binary_path || (platform === 'windows' ? 'courseforge-mcp.exe' : 'courseforge-mcp');
  const coursesDir = status.courses_dir || './courses';
  const dataDir = status.data_dir || './data';

  const lines: string[] = [];
  lines.push(
    '# Настройка CourseForge MCP для AI-ассистента',
    '',
    'Ты — AI-ассистент разработчика. Твоя задача — подключить MCP-сервер CourseForge к своей среде, чтобы иметь возможность исследовать курсы, читать условия задач, смотреть тесты и запускать код через CourseForge.',
    '',
    '## Параметры сервера CourseForge',
    `- Платформа хоста: ${platform}`,
    `- Статус бинарника: ${status.available ? 'готов к работе' : 'требуется сборка'}`,
    `- Путь к бинарнику: ${binary}`,
    `- Директория курсов: ${coursesDir}`,
    `- Директория данных: ${dataDir}`,
    `- Доступно инструментов: ${status.tools_count || 9}`,
    '',
  );

  if (!status.available) {
    lines.push(
      '## Шаг 1: Сборка MCP-бинарника (если еще не собран)',
      'Выполни в корне репозитория CourseForge команду:',
      '```bash',
      'go build -o ./bin/courseforge-mcp ./cmd/mcp',
      '# Или для глобальной установки в $GOPATH/bin:',
      'go install ./cmd/mcp',
      '```',
      '',
    );
  }

  lines.push(
    '## Конфигурация для клиентов',
    '',
    `### Claude Desktop (${getClaudeConfigPath(platform)})`,
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            command: binary,
            args: [`--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`],
          },
        },
      },
      null,
      2,
    ),
    '```',
    '',
    `### Cursor (${getCursorConfigPath()})`,
    '```json',
    JSON.stringify(
      {
        mcpServers: {
          courseforge: {
            command: binary,
            args: [`--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`],
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
            command: binary,
            args: [`--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`],
          },
        },
      },
      null,
      2,
    ),
    '```',
    '',
    '## Инструкция для агента',
    '1. Добавь секцию `"courseforge"` в свой конфигурационный файл MCP согласно выбранному клиенту.',
    '2. Перезапусти клиент или перезагрузи конфигурацию MCP.',
    '3. Вызови инструмент `list_courses` для проверки связи с платформой.',
    '4. При успехе подтверди готовность кратко: «MCP CourseForge успешно подключен. Доступно 9 инструментов.»',
  );

  return lines.join('\n');
}
