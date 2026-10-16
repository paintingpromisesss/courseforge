import { describe, it, expect } from 'vitest';
import { relativeTimeRu } from './gitTime';

const NOW = Date.UTC(2026, 9, 17, 12, 0, 0); // 17.10.2026 12:00 UTC

describe('relativeTimeRu', () => {
  it('just now for sub-minute', () => {
    expect(relativeTimeRu(new Date(NOW - 10_000).toISOString(), NOW)).toBe('только что');
  });

  it('minute for under two minutes', () => {
    expect(relativeTimeRu(new Date(NOW - 60_000).toISOString(), NOW)).toBe('минуту назад');
  });

  it('minutes under an hour', () => {
    expect(relativeTimeRu(new Date(NOW - 3 * 60_000).toISOString(), NOW)).toBe('3 мин назад');
    expect(relativeTimeRu(new Date(NOW - 44 * 60_000).toISOString(), NOW)).toBe('44 мин назад');
  });

  it('hour and hours under a day', () => {
    expect(relativeTimeRu(new Date(NOW - 60 * 60_000).toISOString(), NOW)).toBe('час назад');
    expect(relativeTimeRu(new Date(NOW - 5 * 60 * 60_000).toISOString(), NOW)).toBe('5 ч назад');
  });

  it('yesterday for under two days', () => {
    expect(relativeTimeRu(new Date(NOW - 24 * 60 * 60_000).toISOString(), NOW)).toBe('вчера');
  });

  it('days then calendar date', () => {
    expect(relativeTimeRu(new Date(NOW - 3 * 24 * 60 * 60_000).toISOString(), NOW)).toBe('3 дн назад');
    expect(relativeTimeRu(new Date(NOW - 60 * 24 * 60 * 60_000).toISOString(), NOW)).toBe('18.08.26');
  });

  it('empty string for unparseable input', () => {
    expect(relativeTimeRu('not a date', NOW)).toBe('');
  });
});
