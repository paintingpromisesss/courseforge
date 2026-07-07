//go:build !windows

package runner

// defaultDrivers returns the built-in language drivers for Unix-like hosts.
// The runner writes the submitted code to "solution<ext>" (Java: "Solution<ext>",
// since Java requires the public class name to match the file name) and, in
// task mode, the test to "solution<test_ext>" (Java: "SolutionTest<ext>") —
// matching how course test files include/import the submitted code. Commands
// reference {file}/{testfile}/{dir} placeholders, expanded to those paths.
// All commands are user-editable from Settings, so the host-specific paths
// below (junit jars, etc.) are sensible defaults, not hard requirements.
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
			RunCmd:  []string{"python3", "{file}"},
			TestCmd: []string{"python3", "-m", "pytest", "-v", "{testfile}"},
			Ext:     ".py",
			TestExt: "_test.py",
		},
		"javascript": {
			RunCmd:  []string{"node", "{file}"},
			TestCmd: []string{"npx", "--yes", "mocha", "--reporter", "tap", "{testfile}"},
			Ext:     ".js",
			TestExt: "_test.js",
		},
		"cpp": {
			RunCmd:  []string{"sh", "-c", "g++ -std=c++17 -O2 -o cf-run {file} && ./cf-run"},
			TestCmd: []string{"sh", "-c", "g++ -std=c++17 -O2 -o cf-test {testfile} -lgtest -lgtest_main -pthread && ./cf-test"},
			Ext:     ".cpp",
			TestExt: "_test.cpp",
		},
		"java": {
			RunCmd: []string{"sh", "-c", "javac {file} && java Solution"},
			TestCmd: []string{"sh", "-c",
				"javac -cp /usr/share/java/junit-platform-console-standalone.jar {file} {testfile} && " +
					"java -jar /usr/share/java/junit-platform-console-standalone.jar execute -cp . --scan-classpath --disable-ansi-colors --details=tree"},
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
			RunCmd:      []string{"psql", "-v", "ON_ERROR_STOP=1", "-f", "schema.sql", "-f", "{file}"},
			TestCmd:     []string{"sh", "-c", "psql -v ON_ERROR_STOP=1 -f schema.sql -f {file} && pg_prove -v {testfile}"},
			Ext:         ".sql",
			TestExt:     "_test.sql",
			NeedsSchema: true,
			// Tasks override this with their own DDL/seed data via task.yaml's
			// init_files; the empty default keeps RunCmd/TestCmd's `-f schema.sql`
			// working (and this package's own tests runnable) even without one.
			InitFiles: map[string]string{"schema.sql": ""},
		},
	}
}
