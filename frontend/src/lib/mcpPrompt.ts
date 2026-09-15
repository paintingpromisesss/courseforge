import type { MCPStatusResponse } from '../api/types';

export function buildMCPSetupPrompt(status: MCPStatusResponse): string {
  const platform = status.platform || 'windows';
  const coursesDir = status.courses_dir || './courses';
  const dataDir = status.data_dir || './data';
  const command = status.command || status.binary_path || (platform === 'windows' ? 'courseforge.exe' : 'courseforge');
  const args = status.args && status.args.length > 0 ? status.args : ['mcp', `--courses-dir=${coursesDir}`, `--data-dir=${dataDir}`];
  const sseUrl = status.sse_url || `http://${status.host || '127.0.0.1'}:${status.port || 8080}/api/mcp/sse`;

  const lines: string[] = [];
  lines.push(
    '# Настройка MCP-сервера CourseForge',
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

  lines.push(
    '## Параметры подключения:',
    '- **Имя сервера:** `courseforge`',
    `- **Команда (command):** \`${command}\``,
    `- **Аргументы (args):** \`${JSON.stringify(args)}\``,
    `- **Либо SSE URL (url):** \`${sseUrl}\``,
    '',
    '## Задача:',
    '1. Добавь сервер `courseforge` в конфигурацию MCP твоего окружения (используя command и args для stdio, либо url для SSE).',
    '2. Перезагрузи / обнови список MCP-серверов, если это требуется.',
    '3. Вызови инструмент `list_courses` для проверки связи с платформой.',
    '4. Кратко подтверди готовность: «MCP-сервер CourseForge успешно подключен».',
  );

  return lines.join('\n');
}
