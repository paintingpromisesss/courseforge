import { describe, it, expect } from 'vitest';
import { parseTestOutput } from './parseTests';

describe('parseTestOutput', () => {
  it('go: parses -v PASS/FAIL', () => {
    const out = [
      '=== RUN   TestAdd',
      '--- PASS: TestAdd (0.00s)',
      '=== RUN   TestSub',
      '    main_test.go:9: want 0 got 1',
      '--- FAIL: TestSub (0.00s)',
      'FAIL',
    ].join('\n');
    const r = parseTestOutput('go', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests.find((t) => t.name === 'TestSub')?.detail).toContain('want 0 got 1');
  });

  it('go: scores t.Run subtests individually, not the collapsed parent line', () => {
    const out = [
      '=== RUN   TestReverseString',
      '=== RUN   TestReverseString/case_1',
      '    main_test.go:38: got "kk", want "olleh"',
      '=== RUN   TestReverseString/case_2',
      '--- FAIL: TestReverseString (0.00s)',
      '    --- FAIL: TestReverseString/case_1 (0.00s)',
      '    --- PASS: TestReverseString/case_2 (0.00s)',
      'FAIL',
    ].join('\n');
    const r = parseTestOutput('go', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests.find((t) => t.name === 'TestReverseString/case_1')?.detail).toContain('want "olleh"');
  });

  it('go: keeps a standalone top-level failure next to passing subtests', () => {
    const out = [
      '=== RUN   TestValidateInput',
      '    main_test.go:12: got true, want false',
      '--- FAIL: TestValidateInput (0.00s)',
      '=== RUN   TestCases',
      '=== RUN   TestCases/case_1',
      '=== RUN   TestCases/case_2',
      '--- PASS: TestCases (0.00s)',
      '    --- PASS: TestCases/case_1 (0.00s)',
      '    --- PASS: TestCases/case_2 (0.00s)',
      'FAIL',
    ].join('\n');
    const r = parseTestOutput('go', out, '', 1);
    expect(r.total).toBe(3);
    expect(r.passed).toBe(2);
    expect(r.tests.find((t) => t.name === 'TestValidateInput')?.passed).toBe(false);
    expect(r.tests.find((t) => t.name === 'TestCases')).toBeUndefined();
  });

  it('go: "no tests to run" must not be shown as a pass', () => {
    const out = 'testing: warning: no tests to run\nPASS\nok  \tplayground\t(cached) [no tests to run]\n';
    const r = parseTestOutput('go', out, '', 1);
    expect(r.passed).toBe(0);
  });

  it('go: captures t.Logf detail for passing subtests too, not just failures', () => {
    const out = [
      '=== RUN   TestReverseString',
      '=== RUN   TestReverseString/case_1',
      '    main_test.go:37: input="hello" got="kk" want="olleh"',
      '    main_test.go:39: mismatch: input="hello" got="kk" want="olleh"',
      '=== RUN   TestReverseString/case_2',
      '    main_test.go:37: input="kk" got="kk" want="kk"',
      '--- FAIL: TestReverseString (0.00s)',
      '    --- FAIL: TestReverseString/case_1 (0.00s)',
      '    --- PASS: TestReverseString/case_2 (0.00s)',
      'FAIL',
    ].join('\n');
    const r = parseTestOutput('go', out, '', 1);
    expect(r.tests.find((t) => t.name === 'TestReverseString/case_1')?.detail).toContain('mismatch: input="hello"');
    expect(r.tests.find((t) => t.name === 'TestReverseString/case_2')?.detail).toContain('input="kk" got="kk" want="kk"');
  });

  it('python3: parses pytest -v with failure detail', () => {
    const out = [
      'collected 2 items',
      '',
      'main_test.py::test_add PASSED                                            [ 50%]',
      'main_test.py::test_sub FAILED                                            [100%]',
      '',
      '=================================== FAILURES ===================================',
      '___________________________________ test_sub ___________________________________',
      '',
      '>       assert sub(2, 1) == 0',
      'E       assert 1 == 0',
      '',
      '=========================== short test summary info ============================',
      'FAILED main_test.py::test_sub - assert 1 == 0',
    ].join('\n');
    const r = parseTestOutput('python3', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests.map((t) => t.name)).toEqual(['test_add', 'test_sub']);
    expect(r.tests[1].detail).toContain('assert 1 == 0');
  });

  it('javascript: parses flat mocha TAP', () => {
    const out = [
      'TAP version 13',
      'ok 1 Solution add',
      'not ok 2 Solution sub',
      '  ---',
      '  message: expected 1 to equal 0',
      '  ---',
      '1..2',
      '# pass 1',
      '# fail 1',
    ].join('\n');
    const r = parseTestOutput('javascript', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests[0].name).toBe('Solution add');
    expect(r.tests[1].detail).toContain('expected 1 to equal 0');
  });

  it('javascript: parses nested node:test TAP, skipping the describe rollup line', () => {
    const out = [
      'TAP version 13',
      '# Subtest: mergeTwoLists',
      '    # Subtest: example 1',
      '    ok 1 - example 1',
      '      ---',
      "      duration_ms: 1.3",
      "      type: 'test'",
      '      ...',
      '    # Subtest: example 2',
      '    not ok 2 - example 2',
      '      ---',
      "      duration_ms: 0.2",
      "      type: 'test'",
      "      error: |-",
      '        expected 1 to equal 0',
      '      ...',
      '    1..2',
      'ok 1 - mergeTwoLists',
      '  ---',
      '  duration_ms: 3.9',
      "  type: 'suite'",
      '  ...',
      '1..1',
    ].join('\n');
    const r = parseTestOutput('javascript', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests.map((t) => t.name)).toEqual(['example 1', 'example 2']);
    expect(r.tests[1].detail).toContain('expected 1 to equal 0');
  });

  it('cpp: parses gtest without double-counting the summary list', () => {
    const out = [
      '[==========] Running 2 tests from 1 test suite.',
      '[ RUN      ] SolutionTest.Add',
      '[       OK ] SolutionTest.Add (0 ms)',
      '[ RUN      ] SolutionTest.Sub',
      'main_test.cpp:10: Failure',
      'Expected equality of these values',
      '[  FAILED  ] SolutionTest.Sub (0 ms)',
      '[==========] 2 tests from 1 test suite ran. (0 ms total)',
      '[  PASSED  ] 1 test.',
      '[  FAILED  ] 1 test, listed below:',
      '[  FAILED  ] SolutionTest.Sub',
      '',
      ' 1 FAILED TEST',
    ].join('\n');
    const r = parseTestOutput('cpp', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests[1].detail).toContain('Expected equality');
  });

  it('java: parses JUnit 5 console tree with a failure (no parens after method name)', () => {
    const out = [
      '╷',
      '├─ JUnit Platform Suite ✔',
      '├─ JUnit Jupiter ✔',
      '│  └─ SolutionTest ✔',
      '│     ├─ addBad ✘ expected: <9> but was: <3>',
      '│     └─ addWorks ✔',
      '└─ JUnit Vintage ✔',
      '',
      'Test run finished after 54 ms',
      '[         2 tests successful ]',
      '[         1 tests failed     ]',
    ].join('\n');
    const r = parseTestOutput('java', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    const failed = r.tests.find((t) => !t.passed);
    expect(failed?.name).toBe('addBad');
    expect(failed?.detail).toContain('expected: <9> but was: <3>');
  });

  it('java: parses JUnit 5 all-pass tree', () => {
    const out = [
      '├─ JUnit Jupiter ✔',
      '│  └─ SolutionTest ✔',
      '│     ├─ addWorks ✔',
      '│     └─ subWorks ✔',
      '[         2 tests successful ]',
      '[         0 tests failed     ]',
    ].join('\n');
    const r = parseTestOutput('java', out, '', 0);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(2);
    expect(r.tests.map((t) => t.name)).toEqual(['addWorks', 'subWorks']);
  });

  it('java: falls back to summary counts when no tree', () => {
    const out = ['[         3 tests successful ]', '[         1 tests failed     ]'].join('\n');
    const r = parseTestOutput('java', out, '', 1);
    expect(r.total).toBe(4);
    expect(r.passed).toBe(3);
  });

  it('csharp: ignores build-log noise lines starting with "Failed to"', () => {
    const out = [
      'Failed to load prune package data from PrunePackageData folder, loading from targeting packs instead',
      'Test run for /tmp/app.dll (.NETCoreApp,Version=v10.0)',
      '  Passed TestAdd [< 1 ms]',
      '',
      'Passed!  - Failed:     0, Passed:     1, Skipped:     0, Total:     1 - main.dll',
    ].join('\n');
    const r = parseTestOutput('csharp', out, '', 0);
    expect(r.total).toBe(1);
    expect(r.passed).toBe(1);
    expect(r.tests[0].name).toBe('TestAdd');
  });

  it('csharp: parses dotnet test -v n per-test lines with real names', () => {
    const out = [
      'Test run for /tmp/app.dll (.NETCoreApp,Version=v10.0)',
      '  Passed TestAdd [< 1 ms]',
      '  Failed TestSub [1 ms]',
      '  Error Message:',
      '     Assert.That(actual, Is.EqualTo(0))',
      '  Expected: 0',
      '  Stack Trace:',
      '     at SolutionTest.TestSub() in /tmp/solution_test.cs:line 10',
      '',
      '',
      'Failed!  - Failed:     1, Passed:     1, Skipped:     0, Total:     2 - main.dll',
    ].join('\n');
    const r = parseTestOutput('csharp', out, '', 1);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(1);
    expect(r.tests.map((t) => t.name)).toEqual(['TestAdd', 'TestSub']);
    expect(r.tests[1].detail).toContain('Assert.That(actual, Is.EqualTo(0))');
  });

  it('csharp: falls back to summary counts when no per-test lines', () => {
    const out = 'Passed!  - Failed:     0, Passed:     2, Skipped:     0, Total:     2 - main.dll';
    const r = parseTestOutput('csharp', out, '', 0);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(2);
  });

  it('falls back to a raw error entry on compile failure', () => {
    const r = parseTestOutput('cpp', '', 'main.cpp:3:1: error: expected ;', 1);
    expect(r.total).toBe(1);
    expect(r.passed).toBe(0);
    expect(r.tests[0].detail).toContain('error: expected');
  });
});
