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
			RunCmd:  []string{"python", "main.py"},
			TestCmd: []string{"python", "-m", "pytest", "-v", "main_test.py"},
			Ext:     ".py",
			TestExt: "_test.py",
		},
		"javascript": {
			RunCmd:  []string{"cmd", "/c", "node main.js"},
			TestCmd: []string{"cmd", "/c", "npx --yes mocha --reporter tap main_test.js"},
			Ext:     ".js",
			TestExt: "_test.js",
		},
		"cpp": {
			RunCmd:  []string{"cmd", "/c", "g++ -std=c++17 -O2 -o cf-run.exe main.cpp && cf-run.exe"},
			TestCmd: []string{"cmd", "/c", "g++ -std=c++17 -O2 -o cf-test.exe main_test.cpp -lgtest -lgtest_main && cf-test.exe"},
			Ext:     ".cpp",
			TestExt: "_test.cpp",
		},
		"java": {
			RunCmd:  []string{"cmd", "/c", "javac main.java && java Main"},
			TestCmd: []string{"cmd", "/c", "javac -cp junit-platform-console-standalone.jar main.java main_test.java && java -jar junit-platform-console-standalone.jar execute -cp . --scan-classpath --disable-ansi-colors --details=tree"},
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
	}
}
