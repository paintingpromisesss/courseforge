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

  it('javascript: parses mocha TAP', () => {
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

  it('java: parses JUnit 5 console tree with a failure', () => {
    const out = [
      '╷',
      '├─ JUnit Platform Suite ✔',
      '├─ JUnit Jupiter ✔',
      '│  └─ SolutionTest ✔',
      '│     ├─ addBad() ✘ expected: <9> but was: <3>',
      '│     └─ addWorks() ✔',
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
      '│     ├─ addWorks() ✔',
      '│     └─ subWorks() ✔',
      '[         2 tests successful ]',
      '[         0 tests failed     ]',
    ].join('\n');
    const r = parseTestOutput('java', out, '', 0);
    expect(r.total).toBe(2);
    expect(r.passed).toBe(2);
  });

  it('java: falls back to summary counts when no tree', () => {
    const out = ['[         3 tests successful ]', '[         1 tests failed     ]'].join('\n');
    const r = parseTestOutput('java', out, '', 1);
    expect(r.total).toBe(4);
    expect(r.passed).toBe(3);
  });

  it('csharp: parses dotnet test summary', () => {
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
