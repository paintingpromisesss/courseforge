# GitHub-импорт курсов и облачная синхронизация — дизайн

Дата: 2026-09-27
Ветка: `feat/github-sync` (две фазы, промежуточные коммиты, пуш — только после одобрения)

## Цели

1. **Импорт курсов из GitHub**: по ссылке на репозиторий подтягивать курс/каталог в `courses/`. Переключение веток (при этом прогресс не трогается и лежит независимо). Приватные репозитории — через Personal Access Token.
2. **Облачное хранилище курсов и прогресса на GitHub** (в духе obsidian-git): авторизация, настройка репозитория и ветки, триггеры коммитов/пушей, история с возможностью откатиться на любой коммит, восстановление импортированных курсов на новом устройстве.

## Принятые решения

| Решение | Выбор | Альтернатива |
|---|---|---|
| Git-движок | Системный git CLI (подпроцесс) | go-git — слаб в merge/rebase |
| Auth GitHub | PAT (fine-grained), хранится в data-dir | OAuth Device Flow — можно добавить позже, конфиг закладываем |
| Архитектура облака | Зеркало (snapshot-on-remote) в `data-dir/sync/` | Vault (courses/ = git-репо) — инвазивно, конфликты merge в UI |
| Конфликты устройств | Last-push-wins by construction + полная история для отката | Union-merge прогресса / интерактивный выбор |
| Фазирование | 2 фазы в одной ветке, коммит между фазами | Отдельные ветки |

## Фаза 0 (фундамент): progress.json переезжает в data-dir

Сейчас `FileProgressRepository` пишет `{coursesDir}/{courseDir}/progress.json` (repo/progress.go:125).
Новое расположение: `{dataDir}/progress/{courseDir}/progress.json` — структура каталогов зеркалит courseDir (включая вложенные `catalog/course`).

Миграция: при старте сервера (в DI) — если старый файл существует и нового нет, перенести его (с удалением старого). Идемпотентно.

Причина: (а) переключение веток в импортированном клоне не должно стирать/конфликтовать с прогрессом; (б) облачный синк отделяет прогресс от контента курсов.

Затрагивает: `FileProgressRepository` (конструктор получает dataDir), `di.go`, тесты progress.go.

## Общий компонент: git-обёртка

Новый пакет `backend/internal/infrastructure/git`:

- Запуск git CLI через `exec.CommandContext` с таймаутом (дефолт 60s на операцию, clone — 5min).
- **Токен передаётся через env-переменные git**: `GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0=http.https://github.com/.extraHeader`, `GIT_CONFIG_VALUE_0=Authorization: Bearer <PAT>`. Токен не попадает ни в argv (не виден в списке процессов), ни в `.git/config`, ни в credential-хранилище.
- Один глобальный `sync.Mutex` на сервис (все git-операции последовательны). `ponytail:` per-repo locks, если throughput станет проблемой.
- Операции: `Clone(url, dir, branch)`, `Fetch(dir)`, `ListRemoteBranches(dir)`, `Checkout(dir, branch)` (fetch + hard-align на `origin/<branch>` + clean untracked вне `.git`), `PullFF(dir)` (fast-forward only), `HeadCommit(dir)`, `Log(dir, limit)`, `Status(dir)` (porcelain), `Init(dir)`, `CommitAll(dir, msg)`, `Push(dir)`, `ShowFiles(dir, commit)`.
- Ошибка = типизированная структура с stderr гита (для UI-сообщений: «репо не найдено», «нужна авторизация» — по кодам/тексту git).

Хранилище токена: `{dataDir}/git_auth.json` — по образцу `ai_config.json`:
```json
{ "token": "...", "username": "" }
```
API: `GET /api/git/auth` (возвращает `{configured: bool, token_masked: "ghp_…abcd"}`), `PATCH /api/git/auth` (установка/смена), `DELETE /api/git/auth` (удаление). Токен никогда не возвращается целиком.

## Фаза 1: импорт курсов из GitHub

### Backend

Метаданные источников: `{dataDir}/course_sources.json` (вне папки курса, чтобы working tree клона оставался чистым):
```json
{ "courses": { "<courseDir>": { "repo": "https://github.com/user/repo", "branch": "main", "commit": "abc123", "imported_at": "..." } } }
```

Эндпоинты:

- `POST /api/git/import` — `{url, branch?}`.
  Поток: валидация URL (https, host github.com) → clone во временную папку (с токеном, если есть; без токена — публичные репо) → ищем `course.yaml` или `catalog.yaml` **в корне репо** → `courseparser.LoadOne/LoadCatalogOne` для валидации → проверка коллизии slug → move в `courses/<slug>` (вместе с `.git`) → регистрация через существующий `loadAndRegisterCourse/loadAndRegisterCatalog` → запись в `course_sources.json`.
  **Ограничение v1**: манифест только в корне репо; иначе 400 с понятным сообщением. Апгрейд-путь: sparse-checkout + поле `path` в метаданных.
  Ошибки clone: 401/403 → сообщение «репозиторий приватный или недоступен — добавьте токен в настройках».
- `GET /api/courses/{courseSlug}/git/branches` — список remote-веток + текущая. 404, если курс не импортирован из git.
- `POST /api/courses/{courseSlug}/git/checkout` — `{branch}`: fetch + hard-align на `origin/<branch>`, обновить commit в метаданных, **перегрузить курс в реестре** (re-parse). Прогресс не трогается (он в data-dir).
- `POST /api/courses/{courseSlug}/git/pull` — ff-only pull текущей ветки + re-parse + обновление commit в метаданных.
- `GET /api/courses/{courseSlug}/git/status` — текущая ветка/commit, есть ли изменения в working tree, behind/ahead.

После checkout/pull курс может стать невалидным (битые YAML на новой ветке) — тогда операцию откатываем на предыдущий commit и возвращаем 422 с ошибками парсера.

### Frontend

- CoursesPage: кнопка «Импорт из GitHub» → диалог (URL, ветка опционально, чекбокс «приватный — нужен токен» ведёт в настройки). Прогресс-состояние во время clone.
- CoursePage: панель источника (только для git-курсов) — текущая ветка, выпадающий список веток + Checkout, кнопка Pull, последний commit.
- SettingsPanel: новая секция «GitHub» — поле токена (маска после сохранения), проверка токена (`GET https://api.github.com/user`), удаление токена.
- `api/client.ts`: новые вызовы (camelCase query / snake_case body — по конвенции проекта).

## Фаза 2: облачная синхронизация (зеркало)

### Модель: snapshot-on-remote

Зеркало: `{dataDir}/sync/repo/` — обычный клон пользовательского репо. **Никакого merge никогда**: каждый push пересобирает снимок локального состояния поверх удалённого. Конфликты устройств невозможны by construction; «проигравшая» сторона всегда может откатиться через историю.

Структура облачного репо:
```
courses/<slug>/...          # файлы курсов (без .git импортированных клонов)
progress/<courseDir>.json   # все progress.json
sources.json                # course_sources.json (для восстановления импортов)
```

Конфиг: `{dataDir}/sync_config.json`:
```json
{
  "remote_url": "https://github.com/user/cf-vault",
  "branch": "main",
  "triggers": { "on_progress": true, "interval_min": 0, "on_startup_pull": true },
  "last_sync": "..."
}
```

### Sync-сервис (`internal/application/service` или `internal/infrastructure/sync`)

- `Push()`: mutex → ensure clone (init+remote, если нет) → fetch → hard-align на `origin/<branch>` (или root commit) → очистить содержимое (кроме `.git`) → скопировать курсы (пропуская вложенные `.git`) + прогресс + sources.json → commit (`sync: <hostname> <timestamp>`, автор из токена/user) → push. При отклонении push (кто-то запушил раньше) — retry цикла align→snapshot→push до 3 раз.
- `Pull()`: fetch → hard-align на `origin/<branch>` → copy-back: курсы в `courses/` (заменяя содержимое папок, **не трогая** `.git` импортированных курсов), прогресс в `data/progress/`, sources.json → re-parse всех курсов → применить `sources.json`: курсы-импорты, отсутствующие локально, помечаются «требуется восстановление» (восстановление — явной кнопкой, не автоматически).
  **Семантика copy-back (безопасная): Pull НИКОГДА не удаляет локальные курсы и прогресс.** Курс есть в облаке и локально → обновить; есть в облаке, нет локально → создать; есть локально, нет в облаке → не трогать (он исключён из синка или удалён на другом устройстве — следующий Push его вернёт, last-push-wins). Удаление — только явное локальное действие (существующий DELETE-эндпоинт), после чего Push убирает курс из облака.

### Состав синхронизации (opt-out)

- По умолчанию в синк включены все локальные курсы; git-импорты (курсы с `sources.json`-записью) — всегда исключены из snapshot (их контент живёт в собственном remote, в облако попадает только запись в `sources.json` для восстановления).
- `sync_config.json` содержит список исключений: `"exclude": ["<courseDir>", ...]`. Исключённый курс не коммитится и не обновляется при Pull, но остаётся на диске нетронутым.
- UI «Облако»: список курсов с чекбоксами «синхронизировать».
- `Rollback(commit)`: copy-back файлов из `git show`/checkout данного коммита (как Pull, но из commit) → сразу snapshot-commit «rollback to <short>» → push. История остаётся линейной и синк продолжает работать.
- `History(limit)`: git log зеркало-репо: commit, дата, автор, сообщение + `git show --stat` для деталей.
- `Status()`: configured, last_sync, ahead/behind, локальные изменения с последнего snapshot.

Все операции под одним mutex; HTTP-ручки возвращают 409, если синк уже идёт.

### Триггеры

- **Manual**: кнопка «Синхронизировать» (Push), «Скачать» (Pull).
- **On progress**: после MarkDone/MarkUndone/Reset — debounce 30s → Push. Реализация: ProgressService дёргает коллбэк `OnProgressChanged()`, sync-сервис владеет таймером.
- **Interval**: `interval_min > 0` — тикер Push.
- **On startup**: при включённом `on_startup_pull` — Pull при старте сервера (асинхронно, не блокирует запуск).

### Эндпоинты

- `GET/PATCH /api/sync/config`
- `POST /api/sync/push`, `POST /api/sync/pull`
- `GET /api/sync/status`, `GET /api/sync/history?limit=50`, `GET /api/sync/history/{commit}` (файлы коммита)
- `POST /api/sync/rollback` `{commit}`
- `POST /api/sync/restore-imports` — реклонировать отсутствующие курсы из sources.json

### Frontend

SettingsPanel: секция «Облако» —
- настройка: remote URL, ветка, токен (общий с фазой 1), кнопки Push/Pull, тест подключения;
- триггеры: чекбоксы/поля;
- статус: последний синк, ahead/behind, индикатор «синхронизация…»;
- история: список коммитов (раскрыть — файлы), кнопка «Откатиться» с подтверждением;
- «Восстановить импортированные курсы».

## Обработка ошибок (сводно)

- git отсутствует в PATH → все git-эндпоинты отдают 503 «git not installed», фичи в UI скрыты/заблокированы (проверка один раз при старте + кэш).
- Неверный/отозванный токен → 401 с подсказкой.
- Клон невалиден после checkout → автооткат + 422.
- Push-гонка → retry ×3, затем ошибка «удалённый репо изменился, попробуйте Pull».
- Любая операция синка атомарна для курсов: копируем во временную папку внутри `courses/` и делаем rename (по образцу существующего importFromDir).

## Тестирование

- **git-обёртка**: реальный git в `t.TempDir()` — init локального bare-репо как remote, clone/push/pull/log без GitHub. Таймауты и ошибки — на несуществующих репо.
- **progress-миграция**: юнит-тест (старый файл → новый путь, идемпотентность).
- **snapshot/rollback**: на локальных bare-репо — push → изменение → push → rollback → проверка содержимого; гонка push (второй «клиент» пушит раньше) → retry.
- **handlers**: httptest — import (успех/приватное без токена/невалидный курс), branches/checkout/pull, sync config/status/history/rollback.
- **frontend**: vitest — новые функции client.ts, компоненты диалогов (рендер, состояния ошибок).
- Регенерация swagger (`make swagger`) перед сборкой — все новые ручки аннотированы.

## Явно НЕ делаем (v1)

- OAuth Device Flow (заложено: git_auth.json расширяем без миграции).
- Импорт из подпапки репо (sparse-checkout — апгрейд-путь описан).
- Настоящий merge/diff-UI для конфликтов — заменяется историей + rollback.
- Синхронизация submissions (SQLite) — только курсы, прогресс, источники.
- Автоматическое восстановление импортов при Pull — только явной кнопкой.
