//go:build !windows

package runner

func defaultGoDriver() LangDriver {
	return LangDriver{
		RunCmd:  []string{"go", "run", "."},
		TestCmd: []string{"go", "test", "-v", "."},
		Ext:     ".go",
		TestExt: "_test.go",
		InitFiles: map[string]string{
			"go.mod": "module playground\n\ngo 1.26\n",
		},
	}
}
