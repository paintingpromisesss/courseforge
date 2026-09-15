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
    'Ты — AI-ассистент. Добавь MCP-сервер CourseForge в конфигурацию своего MCP-клиента.',
    '',
    '❗ СТРОГИЕ ПРАВИЛА:',
    '1. НЕ редактируй файлы репозитория и не меняй исходный код проекта (не трогай backend, frontend, courses и т.д.). Это НЕ задача разработки.',
    '2. Твоя цель — добавить MCP-сервер `courseforge` в конфигурационный файл ТВОЕГО СОБСТВЕННОГО окружения/клиента, в котором ты сейчас запущен.',
    '',
  );

  if (!status.enabled) {
    lines.push(
      '⚠️ ВНИМАНИЕ: Сейчас в интерфейсе CourseForge сервер отключен. Перед проверкой включите тоггл в CourseForge: Настройки → MCP-сервер.',
      '',
    );
  }

  if (isSSE) {
    lines.push(
      '## Параметры подключения (SSE HTTP):',
      '- **Имя сервера:** `courseforge`',
      '- **Транспорт:** `sse` (или `remote` / `http`)',
      `- **URL эндпоинта:** \`${sseUrl}\``,
      '',
      '> ℹ️ CourseForge уже запущен и раздает SSE-эндпоинт по сети. Команда запуска процесса не требуется.',
    );
  } else {
    lines.push(
      '## Параметры запуска (stdio):',
      '- **Имя сервера:** `courseforge`',
      '- **Транспорт:** `stdio` (или `local`)',
      `- **Команда (command):** \`${command}\``,
      `- **Аргументы (args):** \`${JSON.stringify(args)}\``,
      `- **Единый массив для запуска:** \`${JSON.stringify(fullCommandArray)}\``,
      '',
      '> ℹ️ Твой клиент будет запускать бинарник в фоне как дочерний процесс.',
    );
  }

  lines.push(
    '',
    '## Справочник: где находится конфиг твоего агента:',
    `- **OpenCode (CLI):** файл \`opencode.json\` (в корне проекта или \`~/.config/opencode/opencode.json\`), корневой ключ \`"mcp"\`. ${
      isSSE
        ? 'Используй `"type": "remote"`, `"url": "' + sseUrl + '"`'
        : 'Используй `"type": "local"`, `"command": ' + JSON.stringify(fullCommandArray) + '`'
    }.`,
    `- **Claude Code (CLI):** выполни команду ${
      isSSE
        ? `\`claude mcp add courseforge --transport sse ${sseUrl}\``
        : `\`claude mcp add courseforge -- ${fullCommandArray.join(' ')}\``
    } либо пропиши в \`~/.claude.json\` / \`.mcp.json\` (ключ \`"mcpServers"\`).`,
    '- **Cursor:** файл `.cursor/mcp.json` (в корне проекта) или `~/.cursor/mcp.json`, ключ `"mcpServers"`.',
    `- **Claude Desktop:** файл \`claude_desktop_config.json\` (${
      platform === 'windows' ? '%APPDATA%\\Claude\\' : '~/Library/Application Support/Claude/'
    }), ключ \`"mcpServers"\`.`,
    `- **Windsurf (Codeium):** файл \`~/.codeium/windsurf/mcp_config.json\`, ключ \`"mcpServers"\`${
      isSSE ? ' (внимание: для SSE используется поле `"serverUrl"`)' : ''
    }.`,
    '- **Cline / Roo Code:** файл настроек расширения `cline_mcp_settings.json` или проектный `.roo/mcp.json`, ключ `"mcpServers"`.',
    '- **Zed:** файл `settings.json` (или `.zed/settings.json`), ключ `"context_servers"`.',
    `- **Antigravity CLI:** файл \`~/.gemini/config/mcp_config.json\`, ключ \`"mcpServers"\`${
      isSSE ? ' (для SSE используется поле `"serverUrl"`)' : ''
    }.`,
    '',
    '## Чеклист действий:',
    '1. Определи, в каком клиенте/агенте ты сейчас работаешь.',
    '2. Найди и обнови соответствующий конфигурационный файл своего окружения (или вызови CLI-команду добавления, если это Claude Code).',
    '3. Перезагрузи / обнови список доступных MCP-инструментов в своём окружении.',
    '4. Вызови инструмент `list_courses` для проверки связи с платформой CourseForge.',
    '5. Кратко доложи пользователю: «MCP-сервер CourseForge успешно подключен к моему окружению».',
  );

  return lines.join('\n');
}
