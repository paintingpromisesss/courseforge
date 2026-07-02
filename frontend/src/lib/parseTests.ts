export interface TestResult {
  name: string;
  passed: boolean;
  detail: string;
}

export interface ParsedResults {
  tests: TestResult[];
  passed: number;
  total: number;
}

function summarize(tests: TestResult[]): ParsedResults {
  return { tests, passed: tests.filter((t) => t.passed).length, total: tests.length };
}

// ── Go: `go test -v` ─────────────────────────────────────────────────────────
export function parseGoTestOutput(stdout: string, stderr: string): ParsedResults {
  const output = stdout + '\n' + stderr;
  const lines = output.split('\n');
  const tests: TestResult[] = [];
  let currentDetail: string[] = [];

  for (const line of lines) {
    if (/^=== RUN\s+\S+/.test(line)) {
      currentDetail = [];
      continue;
    }
    const pass = line.match(/^--- PASS: (\S+)/);
    const fail = line.match(/^--- FAIL: (\S+)/);
    if (pass) {
      tests.push({ name: pass[1], passed: true, detail: currentDetail.join('\n') });
      currentDetail = [];
    } else if (fail) {
      tests.push({ name: fail[1], passed: false, detail: currentDetail.join('\n') });
      currentDetail = [];
    } else {
      const indent = line.match(/^\s{4}(\S.*)$/);
      if (indent) currentDetail.push(indent[1]);
    }
  }

  if (tests.length === 0) {
    const passed = output.includes('ok ') && !output.includes('FAIL');
    tests.push({ name: 'Run', passed, detail: output.trim() });
  }
  return summarize(tests);
}

// ── Python: `pytest -v` ──────────────────────────────────────────────────────
function parsePytest(output: string): ParsedResults | null {
  const tests: TestResult[] = [];
  for (const line of output.split('\n')) {
    const m = line.match(/^(\S+::\S+)\s+(PASSED|FAILED|ERROR|XFAIL|XPASS|SKIPPED)/);
    if (m) {
      const name = m[1].split('::').slice(1).join('::');
      tests.push({ name, passed: m[2] === 'PASSED' || m[2] === 'XFAIL', detail: '' });
    }
  }
  if (tests.length === 0) return null;
  // Attach failure detail from the FAILURES section, keyed by test name.
  const failSection = output.split(/=+ FAILURES =+/)[1];
  if (failSection) {
    for (const t of tests) {
      if (t.passed) continue;
      const re = new RegExp(`_+ ${t.name} _+\\n([\\s\\S]*?)(?=\\n_+ |\\n=+ |$)`);
      const fm = failSection.match(re);
      if (fm) t.detail = fm[1].trim();
    }
  }
  return summarize(tests);
}

// ── JavaScript: `mocha --reporter tap` ───────────────────────────────────────
function parseTap(output: string): ParsedResults | null {
  const lines = output.split('\n');
  const tests: TestResult[] = [];
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/^(ok|not ok)\s+\d+\s*(.*)$/);
    if (!m) continue;
    const passed = m[1] === 'ok';
    const detail: string[] = [];
    if (!passed) {
      // YAML diagnostic block indented beneath the failing point
      for (let j = i + 1; j < lines.length && /^\s/.test(lines[j]); j++) detail.push(lines[j].trim());
    }
    tests.push({ name: m[2].trim() || `test ${tests.length + 1}`, passed, detail: detail.join('\n') });
  }
  if (tests.length === 0) return null;
  return summarize(tests);
}

// ── C++: GoogleTest ──────────────────────────────────────────────────────────
function parseGtest(output: string): ParsedResults | null {
  const lines = output.split('\n');
  const tests: TestResult[] = [];
  let detail: string[] = [];
  for (const line of lines) {
    if (/^\[\s*RUN\s*\]/.test(line)) {
      detail = [];
      continue;
    }
    // Require the `(N ms)` timing so the trailing summary list of `[ FAILED ]`
    // names (printed without timing) is not double-counted.
    const ok = line.match(/^\[\s*OK\s*\]\s+(\S+)\s+\(/);
    const fail = line.match(/^\[\s*FAILED\s*\]\s+(\S+)\s+\(/);
    if (ok) {
      tests.push({ name: ok[1], passed: true, detail: '' });
    } else if (fail) {
      tests.push({ name: fail[1], passed: false, detail: detail.join('\n') });
    } else if (!/^\[/.test(line)) {
      detail.push(line);
    }
  }
  if (tests.length === 0) return null;
  return summarize(tests);
}

// ── Java: JUnit 5 (junit-platform-console-standalone, `--details=tree`) ───────
function parseJUnit(output: string): ParsedResults | null {
  const tests: TestResult[] = [];
  // Tree leaves are the test methods: `methodName() ✔` or
  // `methodName() ✘ <inline failure message>`. Container rows (class/engine
  // names) have no `()` so they are skipped; stack-trace frames carry arguments
  // inside the parens, so the empty `()` guard excludes them too.
  // Horizontal whitespace only ([ \t], not \s) so a trailing ✔ never lets the
  // detail capture spill across the newline into the next tree row.
  const leafRe = /([A-Za-z_$][\w$]*)\(\)[ \t]+(✔|✘)(?:[ \t]+([^\n]*))?/gm;
  let m: RegExpExecArray | null;
  while ((m = leafRe.exec(output))) {
    tests.push({ name: m[1], passed: m[2] === '✔', detail: (m[3] ?? '').trim() });
  }
  if (tests.length > 0) return summarize(tests);

  // Fallback: no per-test tree (e.g. details disabled) — use the summary counts.
  const ok = output.match(/\[\s*(\d+) tests successful/);
  const failed = output.match(/\[\s*(\d+) tests failed/);
  if (ok && failed) {
    const p = Number(ok[1]);
    const f = Number(failed[1]);
    const t: TestResult[] = [];
    for (let i = 0; i < f; i++) t.push({ name: `failure ${i + 1}`, passed: false, detail: '' });
    for (let i = 0; i < p; i++) t.push({ name: `test ${i + 1}`, passed: true, detail: '' });
    if (t.length > 0) return summarize(t);
  }
  return null;
}

// ── C#: NUnit via `dotnet test` ──────────────────────────────────────────────
function parseDotnet(output: string): ParsedResults | null {
  const m = output.match(/Failed:\s*(\d+),\s*Passed:\s*(\d+),\s*Skipped:\s*(\d+),\s*Total:\s*(\d+)/);
  if (!m) return null;
  const failed = Number(m[1]);
  const passed = Number(m[2]);
  const tests: TestResult[] = [];
  // Named failures appear as `  Failed TestName [..]` lines.
  const failNames: string[] = [];
  for (const line of output.split('\n')) {
    const fm = line.match(/^\s*Failed\s+(\S+)/);
    if (fm && fm[1] !== 'Failed') failNames.push(fm[1]);
  }
  for (let i = 0; i < failed; i++) tests.push({ name: failNames[i] ?? `failure ${i + 1}`, passed: false, detail: '' });
  for (let i = 0; i < passed; i++) tests.push({ name: `test ${i + 1}`, passed: true, detail: '' });
  return summarize(tests);
}

// Compile/run error or unrecognized output: surface it as one entry so the user
// still sees the compiler/stderr message.
function genericResult(stdout: string, stderr: string, exitCode: number): ParsedResults {
  const detail = (stderr.trim() || stdout.trim() || '(нет вывода)').trim();
  return summarize([{ name: exitCode === 0 ? 'Выполнено' : 'Ошибка', passed: exitCode === 0, detail }]);
}

const PARSERS: Record<string, (output: string) => ParsedResults | null> = {
  python3: parsePytest,
  javascript: parseTap,
  cpp: parseGtest,
  java: parseJUnit,
  csharp: parseDotnet,
};

/** Parse test output for a language, falling back to a raw error entry. */
export function parseTestOutput(
  language: string,
  stdout: string,
  stderr: string,
  exitCode: number,
): ParsedResults {
  if (language === 'go') return parseGoTestOutput(stdout, stderr);
  const parser = PARSERS[language];
  if (parser) {
    const res = parser(stdout + '\n' + stderr);
    if (res && res.total > 0) return res;
  }
  return genericResult(stdout, stderr, exitCode);
}
