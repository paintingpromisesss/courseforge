import type { CatalogItem, CourseItem, CourseDetail, LangDriver, Progress, RunResp, RunnerStatus, Submission } from './types';

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

export const api = {
  listCourses: () => get<CourseItem[]>('/courses'),
  listCatalogs: () => get<CatalogItem[]>('/catalogs'),
  getCourse: (slug: string) => get<CourseDetail>(`/courses/${slug}`),
  getProgress: (courseSlug: string) => get<Progress>(`/progress/${courseSlug}`),
  markDone: (courseSlug: string, taskSlug: string, done: boolean) =>
    put(`/progress/${courseSlug}/tasks/${taskSlug}`, { done }),

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

  run: (language: string, code: string, testCode: string) =>
    post<RunResp>('/run', { language, code, test_code: testCode, timeout_sec: 30 }),

  listSubmissions: (courseSlug: string, taskSlug: string) =>
    get<Submission[]>(`/submissions?courseSlug=${courseSlug}&taskSlug=${taskSlug}`),

  createSubmission: (body: Omit<Submission, 'id' | 'created_at'>) =>
    post<Submission>('/submissions', body),

  listRunners: () => get<Record<string, LangDriver>>('/runners'),
  patchRunner: (lang: string, body: { run_cmd: string[]; test_cmd: string[] }) =>
    patch<void>(`/runners/${lang}`, body),
  detectRunner: (lang: string) => post<RunnerStatus>(`/runners/${lang}/detect`, {}),

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
};
