import type { MCPStatusResponse } from '../api/types';

export function buildMCPSetupPrompt(status: MCPStatusResponse): string {
  const platform = status.platform || 'windows';
  const coursesDir = status.courses_dir || './courses';
  const dataDir = status.data_dir || './data';
  const command = status.command || status.binary_path || (platform === 'windows' ? 'courseforge.exe' : 'courseforge');
  const args = status.args && status.args.length > 0 ? status.args : ['mcp', `--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`];
  const sseUrl = status.sse_url || `http://${status.host || '127.0.0.1'}:${status.port || 8080}/api/mcp/sse`;
  const isSSE = status.transport === 'sse';

  const lines: string[] = [];
  lines.push(
    `# Настройка MCP-сервера CourseForge (${isSSE ? 'SSE / HTTP' : 'stdio'})`,
    '',
    'Ты — AI-ассистент. Добавь MCP-сервер CourseForge в конфигурацию своего MCP-клиента.',
    '',
  );

  if (!status.enabled) {
    lines.push(
      '⚠️ ВНИМАНИЕ: В интерфейсе CourseForge сервер сейчас выключен. Перед проверкой включите тоггл в CourseForge: Настройки → MCP-сервер.',
      '',
    );
  }

  if (isSSE) {
    lines.push(
      '## Параметры подключения (SSE):',
      '- **Имя сервера:** `courseforge`',
      '- **Тип транспорта:** `sse`',
      `- **URL эндпоинта:** \`${sseUrl}\``,
      '',
      '> ℹ️ Команда запуска не требуется: CourseForge уже запущен и раздает SSE-эндпоинт по сети.',
      '',
      '## Задача:',
      `1. Добавь сервер \`courseforge\` с URL \`${sseUrl}\` в свой конфигурационный файл MCP.`,
      '2. Перезагрузи / обнови список MCP-серверов, если это требуется твоему клиенту.',
      '3. Вызови инструмент `list_courses` для проверки связи с платформой CourseForge.',
      '4. Кратко подтверди готовность: «MCP-сервер CourseForge успешно подключен по SSE».',
      '',
      `*(Примечание: если твой клиент поддерживает только stdio, используй команду \`${command}\` с аргументами ${JSON.stringify(args)})*`,
    );
  } else {
    lines.push(
      '## Параметры запуска (stdio):',
      '- **Имя сервера:** `courseforge`',
      '- **Тип транспорта:** `stdio`',
      `- **Команда (command):** \`${command}\``,
      `- **Аргументы (args):** \`${JSON.stringify(args)}\``,
      '',
      '> ℹ️ Твой MCP-клиент будет напрямую запускать данный бинарник как дочерний процесс при обращении к инструментам.',
      '',
      '## Задача:',
      '1. Добавь сервер `courseforge` в конфигурацию MCP твоего окружения, указав command и args.',
      '2. Перезагрузи / обнови список MCP-серверов, если это требуется твоему клиенту.',
      '3. Вызови инструмент `list_courses` для проверки связи с платформой CourseForge.',
      '4. Кратко подтверди готовность: «MCP-сервер CourseForge успешно подключен через stdio».',
      '',
      `*(Примечание: если твой клиент поддерживает только SSE, используй URL \`${sseUrl}\`)*`,
    );
  }

  return lines.join('\n');
}
