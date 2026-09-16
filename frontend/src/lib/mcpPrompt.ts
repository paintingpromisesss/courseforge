import type { MCPStatusResponse } from '../api/types';

export function buildMCPSetupPrompt(status: MCPStatusResponse): string {
  const platform = status.platform || 'windows';
  const coursesDir = status.courses_dir || './courses';
  const dataDir = status.data_dir || './data';
  const command = status.command || status.binary_path || (platform === 'windows' ? 'courseforge.exe' : 'courseforge');
  const args = status.args && status.args.length > 0 ? status.args : ['mcp', `--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`];
  const sseUrl = status.sse_url || `http://${status.host || '127.0.0.1'}:${status.port || 8080}/api/mcp/sse`;
  const isSSE = status.transport === 'sse';
  const fullCommandArray = [command, ...args];

  const lines: string[] = [];

  lines.push(
    '# Настройка MCP-сервера CourseForge',
    '',
    'Ты — AI-ассистент. Твоя единственная задача — добавить MCP-сервер `courseforge` в конфигурационный файл своего окружения.',
    '',
    '⛔ СТРОГИЕ ЗАПРЕТЫ:',
    '1. НЕ запускай терминальные команды (ls, dir, find, echo и т.д.) для поиска путей и не исследуй систему через shell!',
    '2. НЕ редактируй исходный код репозитория (папки backend, frontend, courses и т.д.).',
    '3. НЕ пытайся вызывать инструменты CourseForge в этой сессии до перезапуска клиента!',
    '',
  );

  if (!status.enabled) {
    lines.push(
      '⚠️ ВНИМАНИЕ: В интерфейсе CourseForge сервер сейчас выключен. Перед проверкой включите тоггл в CourseForge: Настройки → MCP-сервер.',
      '',
    );
  }

  lines.push(
    '## 1. Конфигурация для твоего клиента (используй локальный файл проекта):',
    '',
    '### 👉 Если ты работаешь в OpenCode:',
    'Отредактируй или создай файл `opencode.json` прямо в корне текущего проекта и добавь секцию `"mcp"`:',
    '```json',
    isSSE
      ? JSON.stringify(
          {
            mcp: {
              courseforge: {
                type: 'remote',
                url: sseUrl,
              },
            },
          },
          null,
          2,
        )
      : JSON.stringify(
          {
            mcp: {
              courseforge: {
                type: 'local',
                command: fullCommandArray,
              },
            },
          },
          null,
          2,
        ),
    '```',
    '',
    '### 👉 Если ты работаешь в Cursor или Claude Code:',
    'Отредактируй или создай локальный файл в корне проекта: `.cursor/mcp.json` (для Cursor) или `.mcp.json` (для Claude Code):',
    '```json',
    isSSE
      ? JSON.stringify(
          {
            mcpServers: {
              courseforge: {
                url: sseUrl,
              },
            },
          },
          null,
          2,
        )
      : JSON.stringify(
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
    '*(Для других клиентов: Claude Desktop — глобальный `claude_desktop_config.json`, Windsurf — `~/.codeium/windsurf/mcp_config.json` с полем `"serverUrl"`)*',
    '',
    '## 2. Алгоритм действий (НЕ используй терминал):',
    '1. С помощью инструмента записи/редактирования файлов запиши указанный JSON в соответствующий файл конфигурации (для OpenCode это `opencode.json` в текущей папке).',
    '2. НЕ вызывай никакие команды в shell и НЕ пытайся вызвать инструмент `list_courses` (в OpenCode и большинстве клиентов нет горячей перезагрузки).',
    '3. Сразу ответь пользователю: «MCP-сервер CourseForge добавлен в конфигурацию. Пожалуйста, перезапустите сессию клиента, чтобы инструменты стали доступны».',
  );

  return lines.join('\n');
}
