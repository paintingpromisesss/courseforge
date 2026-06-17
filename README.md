# CourseForge

CourseForge is a self-hosted learning platform that reads local course files,
serves a web UI on `localhost`, and lets users solve coding tasks in the
browser.

## Quick Start

### Windows

```powershell
.\scripts\run.cmd -Port 8080
```

### Linux / WSL

```bash
./scripts/run.sh --port 8080
```

Open [http://localhost:8080](http://localhost:8080).

## Scripts

### Build

```powershell
.\scripts\build.cmd
```

```bash
./scripts/build.sh
```

### Install

```powershell
.\scripts\install.cmd
```

```bash
./scripts/install.sh
```

### Run existing build

```powershell
.\bin\courseforge.exe --port=8080 --courses-dir=.\courses --data-dir=.\data
```

```bash
./bin/courseforge --port=8080 --courses-dir=./courses --data-dir=./data
```

## Notes

- The CLI serves the API and frontend from one Go process.
- Frontend assets are built from `frontend/` into `frontend/dist/`.
- Local course files are read from `./courses` by default.
- Persistent app state goes into `./data` by default.
