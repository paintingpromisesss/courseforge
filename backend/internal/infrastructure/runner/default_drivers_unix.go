//go:build !windows

package runner

// defaultDrivers returns the built-in language drivers for Unix-like hosts.
// The runner writes the submitted code to `main<ext>` and (in task mode) the
// test to `main<test_ext>` inside an isolated temp dir, then runs the driver
// command with that dir as the working directory. Commands reference those
// fixed file names. All commands are user-editable from Settings, so the
// host-specific paths below (junit jars, etc.) are sensible defaults, not hard
// requirements.
func defaultDrivers() map[string]LangDriver {
	return map[string]LangDriver{
		"go": {
			RunCmd:  []string{"go", "run", "."},
			TestCmd: []string{"go", "test", "-v", "."},
			Ext:     ".go",
			TestExt: "_test.go",
			InitFiles: map[string]string{
				"go.mod": "module playground\n\ngo 1.26\n",
			},
		},
		"python3": {
			RunCmd:  []string{"python3", "main.py"},
			TestCmd: []string{"python3", "-m", "pytest", "-v", "main_test.py"},
			Ext:     ".py",
			TestExt: "_test.py",
		},
		"javascript": {
			RunCmd:  []string{"node", "main.js"},
			TestCmd: []string{"npx", "--yes", "mocha", "--reporter", "tap", "main_test.js"},
			Ext:     ".js",
			TestExt: "_test.js",
		},
		"cpp": {
			RunCmd:  []string{"sh", "-c", "g++ -std=c++17 -O2 -o cf-run main.cpp && ./cf-run"},
			TestCmd: []string{"sh", "-c", "g++ -std=c++17 -O2 -o cf-test main_test.cpp -lgtest -lgtest_main -pthread && ./cf-test"},
			Ext:     ".cpp",
			TestExt: "_test.cpp",
		},
		"java": {
			RunCmd: []string{"sh", "-c", "javac main.java && java Main"},
			TestCmd: []string{"sh", "-c",
				"javac -cp /usr/share/java/junit-platform-console-standalone.jar main.java main_test.java && " +
					"java -jar /usr/share/java/junit-platform-console-standalone.jar execute -cp . --scan-classpath --disable-ansi-colors --details=tree"},
			Ext:     ".java",
			TestExt: "_test.java",
		},
		"csharp": {
			RunCmd:  []string{"dotnet", "run", "--project", "."},
			TestCmd: []string{"dotnet", "test", "--nologo", "-v", "q"},
			Ext:     ".cs",
			TestExt: "_test.cs",
			InitFiles: map[string]string{
				"app.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\">\n" +
					"  <PropertyGroup>\n" +
					"    <OutputType>Exe</OutputType>\n" +
					"    <TargetFramework>net10.0</TargetFramework>\n" +
					"    <Nullable>enable</Nullable>\n" +
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
			RunCmd:      []string{"psql", "-v", "ON_ERROR_STOP=1", "-f", "schema.sql", "-f", "{file}"},
			TestCmd:     []string{"sh", "-c", "psql -v ON_ERROR_STOP=1 -f schema.sql -f {file} && pg_prove {testfile}"},
			Ext:         ".sql",
			TestExt:     "_test.sql",
			NeedsSchema: true,
		},
	}
}
