// Minimal shell-like tokenizer so multi-word driver commands (e.g.
// `sh -c "g++ ... && ./cf-run"`) survive an edit round-trip in Settings.
// Not a full POSIX shell — just quotes and backslash escapes.

/** Split a command string into argv, respecting '…' and "…" quoting. */
export function splitArgs(input: string): string[] {
  const args: string[] = [];
  let cur = '';
  let quote: '"' | "'" | null = null;
  let has = false; // a token has started (so "" yields one empty arg)

  for (let i = 0; i < input.length; i++) {
    const c = input[i];
    if (quote) {
      if (c === quote) {
        quote = null;
      } else if (c === '\\' && quote === '"' && i + 1 < input.length) {
        cur += input[++i];
      } else {
        cur += c;
      }
      continue;
    }
    if (c === '"' || c === "'") {
      quote = c;
      has = true;
    } else if (c === '\\' && i + 1 < input.length) {
      cur += input[++i];
      has = true;
    } else if (/\s/.test(c)) {
      if (has) {
        args.push(cur);
        cur = '';
        has = false;
      }
    } else {
      cur += c;
      has = true;
    }
  }
  if (has) args.push(cur);
  return args;
}

/** Join argv back into a command string, double-quoting args that need it. */
export function joinArgs(args: string[]): string {
  return args
    .map((a) => {
      if (a === '') return '""';
      if (/[\s"'\\$`&|;<>(){}*?!#]/.test(a)) {
        return '"' + a.replace(/(["\\$`])/g, '\\$1') + '"';
      }
      return a;
    })
    .join(' ');
}
