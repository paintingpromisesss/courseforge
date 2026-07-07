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
//
// `go test -v` prints every `=== RUN`/failure-message line first, then a
// separate summary block of `--- PASS`/`--- FAIL` lines at the end (indented
// under an unindented parent line for subtests created via t.Run). A single
// left-to-right pass can't attribute detail messages to the right summary
// line, so this collects detail by test name first, then parses the summary
// block (preferring subtest leaves over their parent so a table-driven test
// scores per case instead of collapsing to one).
export function parseGoTestOutput(stdout: string, stderr: string): ParsedResults {
  const output = stdout + '\n' + stderr;
  const lines = output.split('\n');

  const details = new Map<string, string[]>();
  let currentName: string | null = null;
  for (const line of lines) {
    const run = line.match(/^=== RUN\s+(\S+)/);
    if (run) {
      currentName = run[1];
      if (!details.has(currentName)) details.set(currentName, []);
      continue;
    }
    if (/^--- (PASS|FAIL): /.test(line.trim())) {
      currentName = null;
      continue;
    }
    if (currentName) {
      const indent = line.match(/^\s{4}(\S.*)$/);
      if (indent) details.get(currentName)!.push(indent[1]);
    }
  }

  const leaves: TestResult[] = [];
  const tops: TestResult[] = [];
  for (const line of lines) {
    const trimmed = line.trim();
    const pass = trimmed.match(/^--- PASS: (\S+)/);
    const fail = trimmed.match(/^--- FAIL: (\S+)/);
    const m = pass ?? fail;
    if (!m) continue;
    const name = m[1];
    const result: TestResult = { name, passed: !!pass, detail: (details.get(name) ?? []).join('\n') };
    (name.includes('/') ? leaves : tops).push(result);
  }
  const tests = leaves.length > 0 ? leaves : tops;

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
//
// Course tests use node:test's describe/it, so mocha's TAP reporter emits
// *nested* TAP: each `it()` is a leaf `ok N - name` indented under its
// describe block, and the describe block itself repeats as an outer,
// unindented `ok 1 - <describe name>` rollup line. The old regex anchored
// `^` with no leading-whitespace allowance, so it only ever matched that
// single outer rollup line — one entry, not the real per-test names. Leaves
// carry `type: 'test'` in their YAML block, the rollup carries `type:
// 'suite'`; skip the latter so only real tests are counted.
function parseTap(output: string): ParsedResults | null {
  const lines = output.split('\n');
  const tests: TestResult[] = [];
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/^\s*(ok|not ok)\s+\d+\s*-?\s*(.*)$/);
    if (!m) continue;
    const passed = m[1] === 'ok';
    const yaml: string[] = [];
    if (lines[i + 1]?.trim() === '---') {
      for (let j = i + 2; j < lines.length; j++) {
        const t = lines[j].trim();
        if (t === '...' || t === '---') break;
        yaml.push(t);
      }
    }
    if (yaml.includes("type: 'suite'")) continue;
    tests.push({ name: m[2].trim() || `test ${tests.length + 1}`, passed, detail: passed ? '' : yaml.join('\n') });
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
  // Tree leaves are the test methods: `methodName ✔` or
  // `methodName ✘ <inline failure message>` — JUnit Platform Console
  // Launcher 1.13's tree renderer does NOT append `()` after the method
  // name (confirmed against actual --details=tree output), so a regex
  // requiring it never matches and every run falls through to the
  // generic-placeholder-name fallback below.
  // Horizontal whitespace only ([ \t], not \s) so a trailing ✔ never lets the
  // detail capture spill across the newline into the next tree row.
  // Container rows (engine/class names) end in one of a fixed set of tokens —
  // the JUnit engine display names, and "SolutionTest" (the class name the
  // runner always writes the test file as, see runner.go) — so they're
  // excluded by name rather than by requiring `()`, which no longer appears.
  const containerNames = new Set(['Suite', 'Jupiter', 'Vintage', 'SolutionTest']);
  const leafRe = /([A-Za-z_$][\w$]*)[ \t]+(✔|✘)(?:[ \t]+([^\n]*))?/gm;
  let m: RegExpExecArray | null;
  while ((m = leafRe.exec(output))) {
    if (containerNames.has(m[1])) continue;
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

// ── C#: NUnit via `dotnet test -v n` ─────────────────────────────────────────
//
// `-v quiet` (the old flag) prints only the aggregate summary line, no
// per-test names at all, so passing tests were always synthesized as generic
// `test N` placeholders. `-v normal` additionally prints one `Passed Name
// [time]` / `Failed Name [time]` line per test, with the failure's `Error
// Message:`/`Stack Trace:` block indented beneath it — driver now runs `-v n`
// so this can read real names for both outcomes.
function parseDotnet(output: string): ParsedResults | null {
  const lines = output.split('\n');
  const tests: TestResult[] = [];
  for (let i = 0; i < lines.length; i++) {
    // Require the trailing `[time]` so build/restore log lines that happen to
    // start with "Failed to ..." (e.g. NuGet's prune-package-data warning)
    // aren't mistaken for a test result.
    const m = lines[i].match(/^\s*(Passed|Failed)\s+(\S+)\s+\[[^\]]*\]/);
    if (!m) continue;
    const passed = m[1] === 'Passed';
    const detail: string[] = [];
    if (!passed) {
      for (let j = i + 1; j < lines.length && /^\s+\S/.test(lines[j]); j++) detail.push(lines[j].trim());
    }
    tests.push({ name: m[2], passed, detail: detail.join('\n') });
  }
  if (tests.length > 0) return summarize(tests);

  // Fallback: no per-test lines (e.g. verbosity overridden) — use the summary.
  const m = output.match(/Failed:\s*(\d+),\s*Passed:\s*(\d+),\s*Skipped:\s*(\d+),\s*Total:\s*(\d+)/);
  if (!m) return null;
  const failed = Number(m[1]);
  const passed = Number(m[2]);
  const t: TestResult[] = [];
  for (let i = 0; i < failed; i++) t.push({ name: `failure ${i + 1}`, passed: false, detail: '' });
  for (let i = 0; i < passed; i++) t.push({ name: `test ${i + 1}`, passed: true, detail: '' });
  if (t.length > 0) return summarize(t);
  return null;
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
  postgres: parseTap,
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
