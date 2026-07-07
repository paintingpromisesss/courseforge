//go:build windows

package runner

// defaultDrivers returns the built-in language drivers for Windows hosts.
// See the Unix variant for the file-naming contract. Commands are user-editable
// from Settings; the defaults assume the toolchains are on PATH.
func defaultDrivers() map[string]LangDriver {
	return map[string]LangDriver{
		"go": {
			RunCmd:  []string{"cmd", "/c", "go build -o cf-run.exe . && cf-run.exe"},
			TestCmd: []string{"cmd", "/c", "go test -v ."},
			Ext:     ".go",
			TestExt: "_test.go",
			InitFiles: map[string]string{
				"go.mod": "module playground\n\ngo 1.26\n",
			},
		},
		"python3": {
			RunCmd:  []string{"python", "{file}"},
			TestCmd: []string{"python", "-m", "pytest", "-v", "{testfile}"},
			Ext:     ".py",
			TestExt: "_test.py",
		},
		"javascript": {
			RunCmd:  []string{"cmd", "/c", "node {file}"},
			TestCmd: []string{"cmd", "/c", "npx --yes mocha --reporter tap {testfile}"},
			Ext:     ".js",
			TestExt: "_test.js",
		},
		"cpp": {
			RunCmd:  []string{"cmd", "/c", "g++ -std=c++17 -O2 -o cf-run.exe {file} && cf-run.exe"},
			TestCmd: []string{"cmd", "/c", "g++ -std=c++17 -O2 -o cf-test.exe {testfile} -lgtest -lgtest_main && cf-test.exe"},
			Ext:     ".cpp",
			TestExt: "_test.cpp",
		},
		"java": {
			RunCmd:  []string{"cmd", "/c", "javac {file} && java Solution"},
			TestCmd: []string{"cmd", "/c", "javac -cp junit-platform-console-standalone.jar {file} {testfile} && java -jar junit-platform-console-standalone.jar execute -cp . --scan-classpath --disable-ansi-colors --details=tree"},
			Ext:     ".java",
			TestExt: "_test.java",
		},
		"csharp": {
			RunCmd:  []string{"dotnet", "run", "--project", "."},
			TestCmd: []string{"dotnet", "test", "--nologo", "-v", "n"},
			Ext:     ".cs",
			TestExt: "_test.cs",
			InitFiles: map[string]string{
				"app.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\">\n" +
					"  <PropertyGroup>\n" +
					"    <OutputType>Exe</OutputType>\n" +
					"    <TargetFramework>net10.0</TargetFramework>\n" +
					"    <Nullable>enable</Nullable>\n" +
					"    <ImplicitUsings>enable</ImplicitUsings>\n" +
					"  </PropertyGroup>\n" +
					"  <ItemGroup>\n" +
					"    <PackageReference Include=\"Microsoft.NET.Test.Sdk\" Version=\"18.*\" />\n" +
					"    <PackageReference Include=\"NUnit\" Version=\"4.*\" />\n" +
					"    <PackageReference Include=\"NUnit3TestAdapter\" Version=\"6.*\" />\n" +
					"  </ItemGroup>\n" +
					"</Project>\n",
			},
		},
		"postgres": {
			// PostgresManager's Windows implementation (see postgres_windows.go)
			// uses loopback TCP instead of a Unix socket, since Postgres on
			// Windows doesn't support socket files.
			RunCmd:      []string{"psql", "-v", "ON_ERROR_STOP=1", "-f", "schema.sql", "-f", "{file}"},
			TestCmd:     []string{"cmd", "/c", "psql -v ON_ERROR_STOP=1 -f schema.sql -f {file} && pg_prove -v {testfile}"},
			Ext:         ".sql",
			TestExt:     "_test.sql",
			NeedsSchema: true,
			InitFiles:   map[string]string{"schema.sql": ""},
		},
	}
}
