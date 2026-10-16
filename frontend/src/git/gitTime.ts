// Relative time in the app's voice: «3 мин назад», «вчера», «12.03.25».
// now is injectable for tests.
export function relativeTimeRu(iso: string, now: number = Date.now()): string {
  const t = Date.parse(iso);
  if (isNaN(t)) return '';
  const diffSec = Math.round((now - t) / 1000);
  if (diffSec < 45) return 'только что';
  if (diffSec < 90) return 'минуту назад';
  const diffMin = Math.round(diffSec / 60);
  if (diffMin < 45) return `${diffMin} мин назад`;
  if (diffMin < 90) return 'час назад';
  const diffH = Math.round(diffMin / 60);
  if (diffH < 22) return `${diffH} ч назад`;
  const diffD = Math.round(diffH / 24);
  if (diffD <= 1) return 'вчера';
  if (diffD < 45) return `${diffD} дн назад`;

  const d = new Date(t);
  const dd = String(d.getDate()).padStart(2, '0');
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const yy = String(d.getFullYear()).slice(-2);
  return `${dd}.${mm}.${yy}`;
}
