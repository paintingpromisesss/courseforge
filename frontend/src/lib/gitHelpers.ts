// Backend 401 on git import says the repo is private or unavailable — surface a
// hint to Settings → GitHub instead of the raw message.
export function isAuthError(msg: string): boolean {
  const m = msg.toLowerCase();
  return m.includes('private') || m.includes('authentication failed') || m.includes('unavailable');
}

export function shortCommit(hash: string | undefined): string {
  return (hash ?? '').slice(0, 7);
}
