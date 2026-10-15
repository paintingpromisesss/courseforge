import { describe, it, expect } from 'vitest';
import { isAuthError, shortCommit } from './gitHelpers';

describe('isAuthError', () => {
  it('detects private/unavailable repo errors', () => {
    expect(isAuthError('repository is private or unavailable — add a GitHub token in Settings')).toBe(true);
    expect(isAuthError('Authentication failed for https://github.com/x/y')).toBe(true);
  });

  it('ignores unrelated errors', () => {
    expect(isAuthError('course already exists')).toBe(false);
    expect(isAuthError('')).toBe(false);
  });
});

describe('shortCommit', () => {
  it('trims to 7 chars', () => {
    expect(shortCommit('abc1234567890')).toBe('abc1234');
    expect(shortCommit('ab12')).toBe('ab12');
    expect(shortCommit(undefined)).toBe('');
  });
});
