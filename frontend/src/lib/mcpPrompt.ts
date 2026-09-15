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
    '# Настройка MCP-сервера CourseForge',
    '',
    'Ты — AI-ассистент. Добавь MCP-сервер CourseForge в конфигурацию своего MCP-клиента.',
    '',
    'ВАЖНО:',
    '- НЕ редактируй файлы репозитория и не меняй исходный код проекта.',
    '- Твоя задача — настроить именно СВОЁ окружение (клиент, в котором ты запущен), добавив MCP-сервер в конфигурацию инструментов.',
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
      '## Параметры подключения (SSE):',
      '- **name:** `courseforge`',
      '- **transport:** `sse`',
      `- **url:** \`${sseUrl}\``,
      '',
      '> CourseForge уже запущен и раздает SSE-эндпоинт по указанному URL. Команда запуска процесса не требуется.',
    );
  } else {
    lines.push(
      '## Параметры запуска (stdio):',
      '- **name:** `courseforge`',
      `- **command:** \`${command}\``,
      `- **args:** \`${JSON.stringify(args)}\``,
    );
  }

  lines.push(
    '',
    '## Задача:',
    '1. Добавь сервер `courseforge` в конфигурацию MCP твоего окружения, применив структуру конфига, принятую в твоём клиенте.',
    '2. Обнови / перезагрузи список доступных MCP-инструментов.',
    '3. Вызови инструмент `list_courses` для проверки связи с платформой CourseForge.',
    '4. Кратко подтверди готовность: «MCP-сервер CourseForge успешно подключен».',
  );

  return lines.join('\n');
}
