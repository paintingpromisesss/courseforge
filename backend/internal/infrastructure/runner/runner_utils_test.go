package runner

import "testing"

func TestCountTestResultsGoSubtests(t *testing.T) {
	fail := `=== RUN   TestReverseString
=== RUN   TestReverseString/case_1
    main_test.go:38: got "kk", want "olleh"
=== RUN   TestReverseString/case_2
--- FAIL: TestReverseString (0.00s)
    --- FAIL: TestReverseString/case_1 (0.00s)
    --- PASS: TestReverseString/case_2 (0.00s)
FAIL
`
	if passed, total := CountTestResults("go", fail, ""); passed != 1 || total != 2 {
		t.Errorf("got %d/%d, want 1/2", passed, total)
	}

	pass := `=== RUN   TestReverseString
=== RUN   TestReverseString/case_1
=== RUN   TestReverseString/case_2
--- PASS: TestReverseString (0.00s)
    --- PASS: TestReverseString/case_1 (0.00s)
    --- PASS: TestReverseString/case_2 (0.00s)
PASS
ok  	playground	0.003s
`
	if passed, total := CountTestResults("go", pass, ""); passed != 2 || total != 2 {
		t.Errorf("got %d/%d, want 2/2", passed, total)
	}
}

func TestCountTestResultsGoNoTestsToRun(t *testing.T) {
	out := "testing: warning: no tests to run\nPASS\nok  \tplayground\t(cached) [no tests to run]\n"
	if passed, total := CountTestResults("go", out, ""); passed != 0 || total != 1 {
		t.Errorf("got %d/%d, want 0/1 (must fail closed)", passed, total)
	}
}

func TestCountTestResultsTapNestedNodeTest(t *testing.T) {
	out := `TAP version 13
# Subtest: mergeTwoLists
    # Subtest: example 1
    ok 1 - example 1
      ---
      duration_ms: 1.3
      type: 'test'
      ...
    # Subtest: example 2
    not ok 2 - example 2
      ---
      duration_ms: 0.2
      type: 'test'
      ...
    1..2
ok 1 - mergeTwoLists
  ---
  duration_ms: 3.9
  type: 'suite'
  ...
1..1
`
	if passed, total := CountTestResults("javascript", out, ""); passed != 1 || total != 2 {
		t.Errorf("got %d/%d, want 1/2 (suite rollup line must not be double-counted)", passed, total)
	}
}

func TestCountTestResultsJUnitTreeNoParens(t *testing.T) {
	out := `╷
├─ JUnit Platform Suite ✔
├─ JUnit Jupiter ✔
│  └─ SolutionTest ✔
│     ├─ addBad ✘ expected: <9> but was: <3>
│     └─ addWorks ✔
└─ JUnit Vintage ✔

[         2 tests successful ]
[         1 tests failed     ]
`
	if passed, total := CountTestResults("java", out, ""); passed != 1 || total != 2 {
		t.Errorf("got %d/%d, want 1/2 (leaf regex without () must skip container rows)", passed, total)
	}
}

func TestCountTestResultsDotnetIgnoresBuildLogNoise(t *testing.T) {
	out := `Failed to load prune package data from PrunePackageData folder, loading from targeting packs instead
Test run for /tmp/app.dll (.NETCoreApp,Version=v10.0)
  Passed TestAdd [< 1 ms]

Passed!  - Failed:     0, Passed:     1, Skipped:     0, Total:     1 - main.dll
`
	if passed, total := CountTestResults("csharp", out, ""); passed != 1 || total != 1 {
		t.Errorf("got %d/%d, want 1/1 (build log noise line must not be mistaken for a test)", passed, total)
	}
}

func TestCountTestResultsDotnetPerTestLines(t *testing.T) {
	out := `Test run for /tmp/app.dll (.NETCoreApp,Version=v10.0)
  Passed TestAdd [< 1 ms]
  Failed TestSub [1 ms]
  Error Message:
     Assert.That(actual, Is.EqualTo(0))
  Stack Trace:
     at SolutionTest.TestSub() in /tmp/solution_test.cs:line 10

Failed!  - Failed:     1, Passed:     1, Skipped:     0, Total:     2 - main.dll
`
	if passed, total := CountTestResults("csharp", out, ""); passed != 1 || total != 2 {
		t.Errorf("got %d/%d, want 1/2 (dotnet test -v n per-test lines)", passed, total)
	}
}
