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
      if (indent) {
        currentDetail.push(indent[1]);
      }
    }
  }

  if (tests.length === 0) {
    const passed = output.includes('ok ') && !output.includes('FAIL');
    tests.push({ name: 'Run', passed, detail: output.trim() });
  }

  const passed = tests.filter((t) => t.passed).length;
  return { tests, passed, total: tests.length };
}
