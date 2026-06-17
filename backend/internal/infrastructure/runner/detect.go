package runner

import (
	"os/exec"
	"regexp"
	"strings"
)

// DetectStatus is the outcome of probing a language runner on the host.
type DetectStatus string

const (
	StatusOK      DetectStatus = "ok"      // binary found and hello-world test passed
	StatusBroken  DetectStatus = "broken"  // binary found but hello-world test failed
	StatusMissing DetectStatus = "missing" // binary not found in PATH
)

// DetectResult reports whether a language runner is usable on this machine.
type DetectResult struct {
	Status  DetectStatus
	Binary  string // language binary that was looked up (e.g. "go")
	Version string // installed toolchain version, e.g. "go1.22.4"
	Message string // failure detail when Status is broken
}

// langProbe holds the data needed to verify a runner: the binary to find in
// PATH, a hello-world program whose output proves the toolchain works, and how
// to read the installed version.
type langProbe struct {
	bin         string
	code        string
	wantOut     string
	versionArgs []string
	versionRe   *regexp.Regexp
}

var probes = map[string]langProbe{
	"go": {
		bin:         "go",
		code:        "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Print(\"courseforge-ok\") }\n",
		wantOut:     "courseforge-ok",
		versionArgs: []string{"version"},
		versionRe:   regexp.MustCompile(`go[0-9]+(?:\.[0-9]+)*`),
	},
}

// probeVersion runs the binary's version command and extracts a version string.
func probeVersion(path string, p langProbe) string {
	if len(p.versionArgs) == 0 {
		return ""
	}
	out, err := exec.Command(path, p.versionArgs...).Output()
	if err != nil {
		return ""
	}
	s := string(out)
	if p.versionRe != nil {
		if m := p.versionRe.FindString(s); m != "" {
			return m
		}
	}
	return strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
}

// Detect resolves the runner's binary via PATH (exec.LookPath, which honours
// PATHEXT on Windows and mirrors `which`/`where`) and, when a probe exists,
// runs a hello-world program through the driver to confirm it actually works.
func (r *Runner) Detect(lang string) DetectResult {
	p, hasProbe := probes[lang]
	bin := p.bin
	if !hasProbe {
		r.mu.RLock()
		d, ok := r.drivers[lang]
		r.mu.RUnlock()
		if !ok || len(d.RunCmd) == 0 {
			return DetectResult{Status: StatusMissing}
		}
		bin = d.RunCmd[0]
	}

	path, err := exec.LookPath(bin)
	if err != nil {
		return DetectResult{Status: StatusMissing, Binary: bin}
	}
	if !hasProbe {
		// No functional test available — presence in PATH is the best signal.
		return DetectResult{Status: StatusOK, Binary: bin}
	}

	version := probeVersion(path, p)

	res, err := r.Run(RunRequest{Language: lang, Code: p.code})
	if err != nil {
		return DetectResult{Status: StatusBroken, Binary: bin, Version: version, Message: err.Error()}
	}
	if res.TimedOut {
		return DetectResult{Status: StatusBroken, Binary: bin, Version: version, Message: "hello-world test timed out"}
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return DetectResult{Status: StatusBroken, Binary: bin, Version: version, Message: truncate(msg, 300)}
	}
	if p.wantOut != "" && !strings.Contains(res.Stdout, p.wantOut) {
		return DetectResult{Status: StatusBroken, Binary: bin, Version: version, Message: "unexpected output: " + truncate(res.Stdout, 200)}
	}
	return DetectResult{Status: StatusOK, Binary: bin, Version: version}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
