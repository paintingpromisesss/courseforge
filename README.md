# CourseForge

CourseForge is a self-hosted learning platform that reads local course files,
serves a web UI on `localhost`, and lets users solve coding tasks in the
browser.

## Quick Start

Build the project, then run the generated binary.

### Windows

```powershell
.\scripts\build.ps1
.\bin\courseforge.exe --port=8080 --courses-dir=.\courses --data-dir=.\data --frontend-dir=.\frontend\dist
```

### Linux / WSL

```bash
./scripts/build.sh
./bin/courseforge --port=8080 --courses-dir=./courses --data-dir=./data --frontend-dir=./frontend/dist
```

Open [http://localhost:8080](http://localhost:8080).

## Scripts

### Build

```powershell
.\scripts\build.ps1
```

```bash
./scripts/build.sh
```

### Install

```powershell
.\scripts\install.ps1
```

```bash
./scripts/install.sh
```

Install copies the backend binary to a user bin directory. The frontend build
stays in `frontend/dist`, so pass `--frontend-dir` when running an installed
binary outside the repository.

### Run

```powershell
.\bin\courseforge.exe --port=8080 --courses-dir=.\courses --data-dir=.\data --frontend-dir=.\frontend\dist
```

```bash
./bin/courseforge --port=8080 --courses-dir=./courses --data-dir=./data --frontend-dir=./frontend/dist
```

Installed binary example:

```powershell
courseforge.exe --port=8080 --courses-dir=C:\path\to\courses --data-dir=C:\path\to\data --frontend-dir=C:\path\to\courseforge\frontend\dist
```

```bash
courseforge --port=8080 --courses-dir=/path/to/courses --data-dir=/path/to/data --frontend-dir=/path/to/courseforge/frontend/dist
```

## Notes

- The CLI serves the API and frontend from one Go process.
- Frontend assets are built from `frontend/` into `frontend/dist/`.
- Local course files are read from `./courses` by default.
- Persistent app state goes into `./data` by default.
