//go:build windows

package runner

func defaultGoDriver() LangDriver {
	return LangDriver{
		RunCmd:  []string{"cmd", "/c", "go build -o cf-run.exe . && cf-run.exe"},
		TestCmd: []string{"cmd", "/c", "go test -v ."},
		Ext:     ".go",
		TestExt: "_test.go",
		InitFiles: map[string]string{
			"go.mod": "module playground\n\ngo 1.26\n",
		},
	}
}
