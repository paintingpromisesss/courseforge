package runner

import (
	"regexp"
	"strings"
)

// CountTestResults parses test-runner output and returns (passed, total),
// mirroring frontend/src/lib/parseTests.ts so submissions are scored the
// same way the UI displays them — but computed server-side from a server-
// executed run instead of trusting client-reported counts.
func CountTestResults(language, stdout, stderr string) (passed, total int) {
	output := stdout + "\n" + stderr
	switch language {
	case "go":
		return countGoTest(output)
	case "python3":
		if p, t, ok := countPytest(output); ok {
			return p, t
		}
	case "javascript", "postgres":
		if p, t, ok := countTap(output); ok {
			return p, t
		}
	case "cpp":
		if p, t, ok := countGtest(output); ok {
			return p, t
		}
	case "java":
		if p, t, ok := countJUnit(output); ok {
			return p, t
		}
	case "csharp":
		if p, t, ok := countDotnet(output); ok {
			return p, t
		}
	}
	// Unparseable output (no recognized test-runner format) must not be
	// scored as a pass just because the process exited 0 — fail closed.
	return 0, 1
}

var (
	goOkRe     = regexp.MustCompile(`(?m)^ok\s`)
	goFailedRe = regexp.MustCompile(`FAIL`)
)

// countGoTest scores `go test -v` output. Subtests (t.Run) print their
// --- PASS/--- FAIL line indented under an unindented parent line repeating
// the same verdict, e.g.:
//
//	--- FAIL: TestReverseString (0.00s)
//	    --- FAIL: TestReverseString/case_1 (0.00s)
//	    --- PASS: TestReverseString/case_2 (0.00s)
//
// Subtest lines score per-case; a top-level line is counted only when the
// test has no subtests of its own — counting a parent alongside its subtests
// would double-count, but dropping standalone tests would hide their failures.
func countGoTest(output string) (passed, total int) {
	type topResult struct {
		name string
		pass bool
	}
	var pLeaf, fLeaf int
	var tops []topResult
	hasSubtests := map[string]bool{}
	for raw := range strings.SplitSeq(output, "\n") {
		line := strings.TrimSpace(raw)
		var rest string
		var isPass bool
		switch {
		case strings.HasPrefix(line, "--- PASS: "):
			rest, isPass = strings.TrimPrefix(line, "--- PASS: "), true
		case strings.HasPrefix(line, "--- FAIL: "):
			rest, isPass = strings.TrimPrefix(line, "--- FAIL: "), false
		default:
			continue
		}
		name, _, _ := strings.Cut(rest, " ")
		if parent, _, isSubtest := strings.Cut(name, "/"); isSubtest {
			hasSubtests[parent] = true
			if isPass {
				pLeaf++
			} else {
				fLeaf++
			}
		} else {
			tops = append(tops, topResult{name, isPass})
		}
	}
	passed, total = pLeaf, pLeaf+fLeaf
	for _, t := range tops {
		if hasSubtests[t.name] {
			continue
		}
		total++
		if t.pass {
			passed++
		}
	}
	if total > 0 {
		return passed, total
	}
	// "no tests to run" exits 0 with "ok" printed but proves nothing ran —
	// must never be scored as a pass.
	if strings.Contains(output, "no tests to run") {
		return 0, 1
	}
	if goOkRe.MatchString(output) && !goFailedRe.MatchString(output) {
		return 1, 1
	}
	return 0, 1
}

var pytestRe = regexp.MustCompile(`(?m)^\S+::\S+\s+(PASSED|FAILED|ERROR|XFAIL|XPASS|SKIPPED)`)

func countPytest(output string) (passed, total int, ok bool) {
	matches := pytestRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, 0, false
	}
	for _, m := range matches {
		total++
		if m[1] == "PASSED" || m[1] == "XFAIL" {
			passed++
		}
	}
	return passed, total, true
}

var tapLineRe = regexp.MustCompile(`(?m)^\s*(ok|not ok)\s+\d+`)

// countTap scores mocha's TAP reporter output for node:test-authored tests.
// node:test's describe/it nests: each it() is an indented leaf `ok N - name`,
// and the enclosing describe re-emits as an unindented rollup `ok 1 - name`
// with `type: 'suite'` in its YAML block (leaves carry `type: 'test'`).
// Counting every "ok"/"not ok" line double-counts the rollup as an extra
// passing test, so suite rows are excluded.
func countTap(output string) (passed, total int, ok bool) {
	lines := strings.Split(output, "\n")
	matches := tapLineRe.FindAllStringSubmatchIndex(output, -1)
	if len(matches) == 0 {
		return 0, 0, false
	}
	lineOf := func(pos int) int { return strings.Count(output[:pos], "\n") }
	for _, m := range matches {
		lineIdx := lineOf(m[0])
		isSuite := false
		if lineIdx+1 < len(lines) && strings.TrimSpace(lines[lineIdx+1]) == "---" {
			for j := lineIdx + 2; j < len(lines); j++ {
				t := strings.TrimSpace(lines[j])
				if t == "..." || t == "---" {
					break
				}
				if t == "type: 'suite'" {
					isSuite = true
					break
				}
			}
		}
		if isSuite {
			continue
		}
		total++
		if output[m[2]:m[3]] == "ok" {
			passed++
		}
	}
	if total == 0 {
		return 0, 0, false
	}
	return passed, total, true
}

var (
	gtestOkRe   = regexp.MustCompile(`(?m)^\[\s*OK\s*\]\s+\S+\s+\(`)
	gtestFailRe = regexp.MustCompile(`(?m)^\[\s*FAILED\s*\]\s+\S+\s+\(`)
)

func countGtest(output string) (passed, total int, ok bool) {
	p := len(gtestOkRe.FindAllString(output, -1))
	f := len(gtestFailRe.FindAllString(output, -1))
	if p+f == 0 {
		return 0, 0, false
	}
	return p, p + f, true
}

var (
	// JUnit Platform Console Launcher 1.13's --details=tree renderer does not
	// append `()` after leaf method names (confirmed against actual output),
	// so a regex requiring it never matches and every run fell through to the
	// tests-successful/tests-failed summary counts below. Without `()` to
	// exclude container rows, the leaf regex also matches their trailing
	// word — a fixed set of JUnit engine names plus "SolutionTest" (the
	// class name the runner always writes the test file as) — so those are
	// excluded by name instead, mirroring parseTests.ts.
	junitLeafRe      = regexp.MustCompile(`([A-Za-z_$][\w$]*)[ \t]+(✔|✘)`)
	junitContainerRe = regexp.MustCompile(`^(Suite|Jupiter|Vintage|SolutionTest)$`)
	junitSuccessRe   = regexp.MustCompile(`\[\s*(\d+) tests successful`)
	junitFailedRe    = regexp.MustCompile(`\[\s*(\d+) tests failed`)
)

func countJUnit(output string) (passed, total int, ok bool) {
	matches := junitLeafRe.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if junitContainerRe.MatchString(m[1]) {
			continue
		}
		total++
		if m[2] == "✔" {
			passed++
		}
	}
	if total > 0 {
		return passed, total, true
	}
	okM := junitSuccessRe.FindStringSubmatch(output)
	failM := junitFailedRe.FindStringSubmatch(output)
	if okM != nil && failM != nil {
		p := atoi(okM[1])
		f := atoi(failM[1])
		return p, p + f, true
	}
	return 0, 0, false
}

var (
	// Require the trailing `[time]` so build/restore log lines that happen
	// to start with "Failed to ..." (e.g. NuGet's prune-package-data
	// warning) aren't mistaken for a test result.
	dotnetLineRe = regexp.MustCompile(`(?m)^\s*(Passed|Failed)\s+\S+\s+\[[^\]]*\]`)
	dotnetRe     = regexp.MustCompile(`Failed:\s*(\d+),\s*Passed:\s*(\d+),\s*Skipped:\s*(\d+),\s*Total:\s*(\d+)`)
)

// countDotnet mirrors parseDotnet: prefers the per-test `Passed`/`Failed`
// lines dotnet test -v n prints, falling back to the aggregate summary line
// only when those are absent (e.g. verbosity overridden).
func countDotnet(output string) (passed, total int, ok bool) {
	matches := dotnetLineRe.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		total++
		if m[1] == "Passed" {
			passed++
		}
	}
	if total > 0 {
		return passed, total, true
	}
	m := dotnetRe.FindStringSubmatch(output)
	if m == nil {
		return 0, 0, false
	}
	failed, passed := atoi(m[1]), atoi(m[2])
	return passed, passed + failed, true
}

// postgresDDLRe matches statement-starting keywords that create their own
// persistent object (table, view, function, ...). A student submission
// starting with any of these already gives pg_prove's later connection
// something to inspect, so it's left untouched.
var postgresDDLRe = regexp.MustCompile(`(?i)^\s*(CREATE|ALTER|DROP|INSERT|UPDATE|DELETE|TRUNCATE|GRANT|REVOKE)\b`)

// stripLeadingComments removes SQL line comments (--) from the start of
// code so DDL detection works even when students write comments before
// their first statement.
func stripLeadingComments(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "--") {
			return strings.Join(lines[i:], "\n")
		}
	}
	return ""
}

// wrapPostgresQueryAsView wraps a bare SELECT/WITH submission in a view so it
// survives past the psql connection that loads it — pg_prove opens a fresh
// connection to run the test file, and a plain SELECT's result, printed to
// stdout, doesn't exist in the database for that new connection to see.
// DDL submissions already create their own persistent object and are passed
// through as-is.
func wrapPostgresQueryAsView(code string) string {
	trimmed := strings.TrimSpace(stripLeadingComments(code))
	if trimmed == "" || postgresDDLRe.MatchString(trimmed) {
		return code
	}
	return "CREATE OR REPLACE VIEW student_solution AS\n" + trimmed
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
