import type { MCPStatusResponse } from '../api/types';
import { DEFAULT_BACKEND_HOST, DEFAULT_BACKEND_PORT } from './constants';

export function buildMCPSetupPrompt(status: MCPStatusResponse): string {
  const platform = status.platform || 'windows';
  const command = status.command || status.binary_path || (platform === 'windows' ? 'courseforge.exe' : 'courseforge');
  const args = status.args && status.args.length > 0 ? status.args : ['mcp'];
  const sseUrl = status.sse_url || `http://${status.host || DEFAULT_BACKEND_HOST}:${status.port || DEFAULT_BACKEND_PORT}/api/mcp/sse`;
  const isSSE = status.transport === 'sse';
  const fullCommandArray = [command, ...args];

  const lines: string[] = [];

  const currentCommandStr = `${command} ${args.join(' ')}`;
  const buildCmd = platform === 'windows' ? '.\\scripts\\build.ps1' : './scripts/build.sh';

  if (!status.enabled) {
    lines.push(
      '⚠️ ВНИМАНИЕ: В интерфейсе CourseForge сервер сейчас выключен. Перед проверкой включите тоггл в CourseForge: Настройки → MCP-сервер.',
      '',
    );
  }

  if (!isSSE && !status.available) {
    lines.push(
      `⚠️ ВНИМАНИЕ: Бинарник CourseForge ещё не собран на диске (файл по пути \`${command}\` не найден).`,
      `Для работы MCP через stdio сначала соберите бинарник командой \`${buildCmd}\` (или установите релиз), либо переключитесь на транспорт SSE в Настройках CourseForge.`,
      '',
    );
  }

  const contextBinaryLabel = isSSE
    ? 'URL MCP-сервера:'
    : status.available
      ? 'Бинарник уже собран:'
      : 'Бинарник ещё не собран (требуется сборка или установка):';

  const contextBinaryValue = isSSE
    ? sseUrl
    : status.available
      ? currentCommandStr
      : `Для сборки выполните: ${buildCmd}\nКоманда MCP после сборки: ${currentCommandStr}`;

  lines.push(
    '# Подключение MCP-сервера CourseForge',
    '',
    '## Контекст',
    'CourseForge — платформа для самообучения программированию: курсы состоят из задач с теорией, условием, эталонным решением, тест-кейсами и шаблоном. У проекта есть свой MCP-сервер — он встроен в основной бинарник и поднимается командой `mcp`, отдавая агенту доступ к курсам и данным платформы.',
    '',
    contextBinaryLabel,
    contextBinaryValue,
    '',
    '## Задача',
    'Добавь MCP-сервер `courseforge` в конфигурацию того инструмента, в котором ты сейчас работаешь. Я использую один и тот же запрос в разных агентах (OpenCode, Claude Code, Cursor и др.), поэтому конкретный файл и формат зависят от того, кто именно его выполняет — определи это сам и используй подходящий вариант ниже.',
    '',
    '## Как действовать',
    '1. Определи, в каком инструменте ты сейчас запущен, и найди для него нужный конфиг-файл (см. варианты ниже). Если сомневаешься — уточни у меня, не угадывай.',
    '2. Если файла ещё нет — создай его. Если он уже есть и в нём есть другие MCP-серверы — добавь `courseforge` рядом с ними, не трогая остальные записи.',
    '3. Для записи используй свой файловый инструмент (write/edit), а не shell — так надёжнее с экранированием путей на Windows.',
    '4. Задача касается только MCP-конфигурации — код проекта (`backend/`, `frontend/`, `courses/` и т.д.) менять не нужно.',
    '5. После записи проверь, что получившийся JSON валиден.',
    '6. У большинства клиентов нет горячей подгрузки MCP — новый инструмент появится только после перезапуска сессии/клиента. Поэтому вызывать инструменты courseforge прямо сейчас не нужно — это ожидаемо, что они пока недоступны.',
    status.available || isSSE
      ? '7. В конце коротко напиши, какой файл ты изменил и нужен ли перезапуск.'
      : `7. Так как бинарник ещё не собран (available: false), предупреди пользователя, что перед запуском MCP-сервера нужно собрать бинарник командой \`${buildCmd}\`, и укажи, какой файл конфигурации ты изменил.`,
    '',
    '## Варианты конфигурации',
    '',
    '**OpenCode** — файл `opencode.json` в корне проекта, ключ `mcp`:',
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
    isSSE
      ? '**Claude Code** — файл `.mcp.json` в корне проекта, ключ `mcpServers`:'
      : `**Claude Code** — файл \`.mcp.json\` в корне проекта, ключ \`mcpServers\` (либо команда \`claude mcp add courseforge -- ${currentCommandStr}\`):`,
    '```json',
    isSSE
      ? JSON.stringify(
          {
            mcpServers: {
              courseforge: {
                type: 'sse',
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
                type: 'stdio',
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
    '**Cursor** — файл `.cursor/mcp.json` в корне проекта, ключ `mcpServers`, тот же формат, что у Claude Code (поле `type` можно не указывать).',
    '',
    '**Cline / Roo Code** — файл в глобальном хранилище VS Code (`cline_mcp_settings.json` или `mcp_settings.json`; у Roo Code есть ещё и проектный `.roo/mcp.json`), ключ `mcpServers`, формат как у Claude Code.',
    '',
    '**Windsurf** — файл `~/.codeium/windsurf/mcp_config.json`, ключ `mcpServers`, формат как у Claude Code (у этого клиента конфиг только глобальный, проектного нет).',
    '',
    'Если ты работаешь в другом инструменте — используй ту же пару `command`/`args` (или `type: "local"`/`"remote"`, если у твоего клиента другая схема) и найди актуальный путь к конфиг-файлу в его документации.',
  );

  return lines.join('\n');
}
