import type { CatalogItem, CourseItem, CourseDetail, CreateSubmissionReq, LangDriver, Progress, RunnerStatus, Submission, AIConfig, AIMessage, AIModelItem, MCPConfig, MCPStatusResponse, MCPActiveTask } from './types';


const BASE = import.meta.env.VITE_API_URL ?? '/api';

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return res.json();
}

async function getText(path: string): Promise<string> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return res.text();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error ?? String(res.status));
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function patch<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => null);
    throw new Error(err?.error ?? String(res.status));
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function put(path: string, body: unknown): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`PUT ${path} → ${res.status}`);
}

async function del(path: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`DELETE ${path} → ${res.status}`);
}

async function postStream(
  path: string,
  body: unknown,
  onChunk: (text: string) => void,
  onError?: (err: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    });
  } catch (err: unknown) {
    if (signal?.aborted) return;
    const msg = err instanceof Error ? err.message : 'Network error';
    onError?.(msg);
    return;
  }

  if (!res.ok) {
    const err = await res.json().catch(() => null);
    onError?.(err?.error ?? String(res.status));
    return;
  }
  const reader = res.body?.getReader();
  if (!reader) return;
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      if (signal?.aborted) break;
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed === 'data: [DONE]') continue;
        if (trimmed.startsWith('data: ')) {
          const data = trimmed.slice(6).trim();
          try {
            const parsed = JSON.parse(data);
            if (parsed.error) {
              onError?.(parsed.error);
            } else if (parsed.delta) {
              onChunk(parsed.delta);
            }
          } catch {
            // ignore non-JSON or partial chunk
          }
        }
      }
    }
  } catch (err: unknown) {
    if (!signal?.aborted) {
      const msg = err instanceof Error ? err.message : 'Stream interrupted';
      onError?.(msg);
    }
  } finally {
    try {
      reader.releaseLock();
    } catch {
      // ignore
    }
  }
}

export const api = {
  listCourses: () => get<CourseItem[]>('/courses'),
  listCatalogs: () => get<CatalogItem[]>('/catalogs'),
  getCourse: (slug: string) => get<CourseDetail>(`/courses/${slug}`),
  getProgress: (courseSlug: string) => get<Progress>(`/progress/${courseSlug}`),
  markDone: (courseSlug: string, taskSlug: string, done: boolean) =>
    put(`/progress/${courseSlug}/tasks/${taskSlug}`, { done }),
  resetProgress: (courseSlug: string) => del(`/progress/${courseSlug}`),

  getTheory: (courseSlug: string, trackSlug: string, topicSlug: string, unitSlug: string) =>
    getText(`/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/theory`),

  getStatement: (courseSlug: string, trackSlug: string, topicSlug: string, unitSlug: string, taskSlug: string) =>
    getText(`/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${taskSlug}/statement`),

  getTemplate: (courseSlug: string, trackSlug: string, topicSlug: string, unitSlug: string, taskSlug: string, lang: string) =>
    getText(`/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${taskSlug}/template?lang=${lang}`),

  getTests: (courseSlug: string, trackSlug: string, topicSlug: string, unitSlug: string, taskSlug: string, lang: string) =>
    getText(`/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${taskSlug}/tests?lang=${lang}`),

  getSolution: (courseSlug: string, trackSlug: string, topicSlug: string, unitSlug: string, taskSlug: string, lang: string) =>
    getText(`/courses/${courseSlug}/tracks/${trackSlug}/topics/${topicSlug}/units/${unitSlug}/tasks/${taskSlug}/template?lang=${lang}&solution=1`),

  listSubmissions: (courseSlug: string, taskSlug: string) =>
    get<Submission[]>(`/submissions?courseSlug=${courseSlug}&taskSlug=${taskSlug}`),

  createSubmission: (body: CreateSubmissionReq) => post<Submission>('/submissions', body),

  listRunners: () => get<Record<string, LangDriver>>('/runners'),
  listRunnerDefaults: () => get<Record<string, LangDriver>>('/runners/defaults'),
  patchRunner: (lang: string, body: { run_cmd: string[]; test_cmd: string[] }) =>
    patch<void>(`/runners/${lang}`, body),
  detectRunner: (lang: string) => post<RunnerStatus>(`/runners/${lang}/detect`, {}),
  setPostgres: (on: boolean) => post<void>(`/runners/postgres/${on ? 'start' : 'stop'}`, {}),

  // Upload one course/catalog folder. `files` carry their root-prefixed relative
  // paths (e.g. "mycourse/course.yaml"), which is what the backend expects.
  uploadCourseFiles: async (files: { file: File; path: string }[]): Promise<{ slug: string }> => {
    const fd = new FormData();
    // Send all relative paths as a single field (file order) BEFORE the file parts,
    // so the backend can stream parts without buffering the whole form.
    fd.append('paths', JSON.stringify(files.map((f) => f.path)));
    for (const f of files) {
      fd.append('files', f.file);
    }
    const res = await fetch(`${BASE}/courses/upload`, { method: 'POST', body: fd });
    if (!res.ok) {
      const err = await res.json().catch(() => null);
      throw new Error(err?.error ?? String(res.status));
    }
    return res.json();
  },

  deleteCourse: (slug: string) => del(`/courses/${slug}`),
  deleteCatalog: (slug: string, purge = false) => del(`/catalogs/${slug}${purge ? '?purge=1' : ''}`),

  createCatalog: (body: { title: string; description: string }) =>
    post<{ slug: string }>('/catalogs', body),
  patchCatalog: (slug: string, body: { title?: string; description?: string; courses?: string[] }) =>
    patch<void>(`/catalogs/${slug}`, body),

  aiConfig: () => get<AIConfig>('/ai/config'),
  aiSaveConfig: (body: Omit<AIConfig, 'enabled'>) => patch<void>('/ai/config', body),
  aiChatStream: (
    messages: AIMessage[],
    onChunk: (text: string) => void,
    onError?: (err: string) => void,
    signal?: AbortSignal,
    thinking?: boolean,
  ) => postStream('/ai/chat', { messages, thinking }, onChunk, onError, signal),
  aiModels: (body: { provider: string; base_url: string; api_key: string; check_availability?: boolean }) =>
    post<AIModelItem[]>('/ai/models', body),

  mcpConfig: () => get<MCPStatusResponse>('/mcp/config'),
  mcpSaveConfig: (body: Partial<MCPConfig>) => patch<MCPStatusResponse>('/mcp/config', body),
  mcpGetActiveTask: () => get<MCPActiveTask>('/mcp/active-task'),
  mcpSetActiveTask: (body: { course_slug: string; task_slug: string; language?: string }) =>
    put('/mcp/active-task', body),
  mcpClearActiveTask: () => del('/mcp/active-task'),
};


