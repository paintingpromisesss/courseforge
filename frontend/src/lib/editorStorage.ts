const key = (taskSlug: string, lang: string) => `editor:${taskSlug}:${lang}`;

export function loadCode(taskSlug: string, lang: string): string | null {
  return localStorage.getItem(key(taskSlug, lang));
}

export function saveCode(taskSlug: string, lang: string, code: string): void {
  localStorage.setItem(key(taskSlug, lang), code);
}
