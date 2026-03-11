[English](README.md) | [Русский](README.ru.md)

# 🔒 apply_patch_qwen

### Как заставить Qwen перестать переписывать файлы и начать писать нормальные патчи

> "Я хотел кодера, а получил маленького цифрового арестанта, который на четвёртом шаге копает тоннель ложкой."

Строгий Codex-style `apply_patch` для Qwen, Claude Code и других MCP-совместимых coding agents.

## Установка

Одной командой:

```bash
curl -fsSL https://raw.githubusercontent.com/anatoliiii/apply_patch_qwen/main/scripts/install-from-release.sh | sh
```

Установить конкретную версию:

```bash
VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/anatoliiii/apply_patch_qwen/main/scripts/install-from-release.sh | sh
```

### Запасной путь: скачать и запустить

Скачай архив для своей платформы из [Releases](https://github.com/anatoliiii/apply_patch_qwen/releases), затем:

```bash
tar xzf apply_patch_qwen_v*.tar.gz
cd apply_patch_qwen_v*
chmod +x install.sh
./install.sh
```

### Что делает установщик

- ставит `qwen-apply-patch-mcp` и `qwen-apply-patch-tool` в `~/.local/bin`
- обновляет `~/.qwen/settings.json` (конфиг Qwen Code)
- обновляет `~/.claude.json` (конфиг Claude Code)
- регистрирует MCP сервер `strictPatch` для обоих клиентов

После этого перезапусти Qwen Code / Claude Code или открой новую сессию.

### Пути конфигов

| Клиент | Конфиг файл | Что добавляется |
| --- | --- | --- |
| Qwen Code | `~/.qwen/settings.json` | `mcpServers.strictPatch` + исключения инструментов |
| Claude Code | `~/.claude.json` | `mcpServers.strictPatch` |

## Что Это Решает

Многие coding agents воспринимают отклонённый patch как сигнал "ищи другой путь записи":

- `write_file`
- shell redirection
- `tee`, `cat >`, `printf >`
- remote MCP writes через GitLab, issue trackers и другие обходные каналы

`apply_patch_qwen` сужает контракт так, чтобы модель чинила сам patch, а не искала новый маршрут записи в файл.

Этот проект даёт узкий и fail-fast инструмент для менее дисциплинированных моделей:

- один строгий формат патча
- атомарное планирование и commit
- path guard по workspace root
- явные diagnostics для сломанных hunk'ов и `context_mismatch`
- серверные блокировки для типовых обходов вроде `Delete File` + `Add File` на одном и том же пути

## Контракт Патча

Форма запроса:

```json
{
  "patch": "*** Begin Patch\n*** Update File: hello.txt\n@@\n old line\n-old value\n+new value\n*** End Patch\n",
  "dry_run": false
}
```

Правила:

- patch должен начинаться с `*** Begin Patch`
- patch должен заканчиваться `*** End Patch`
- допустимые директивы только `*** Add File:`, `*** Update File:`, `*** Delete File:`, опционально `*** Move to:` и `*** Rename File:`
- unified diff headers вроде `---` / `+++` запрещены
- пути должны быть относительными от workspace root
- абсолютные пути, `~` и `..` запрещены
- `Update File` hunks строгие: каждый hunk начинается с `@@`, а каждая строка внутри начинается с ` `, `+` или `-`

## Валидные Примеры

Создать новый файл:

```text
*** Begin Patch
*** Add File: notes.txt
+hello
+world
*** End Patch
```

Изменить существующий файл:

```text
*** Begin Patch
*** Update File: hello.txt
@@
 old line
-old value
+new value
*** End Patch
```

Переименовать файл без изменения содержимого:

```text
*** Begin Patch
*** Rename File: old.txt
*** Move to: new.txt
*** End Patch
```

## Невалидные Паттерны

Unified diff headers:

```text
--- a/file.txt
+++ b/file.txt
```

Пустой или free-form hunk body:

```text
*** Begin Patch
*** Update File: file.txt
@@
free form text
*** End Patch
```

Замена файла через delete+add:

```text
*** Begin Patch
*** Delete File: file.txt
*** Add File: file.txt
+rewritten
*** End Patch
```

Последний случай режется сервером как `replace_via_delete_add`, и модель получает сообщение, что надо использовать `Update File`.

## Диагностика

Инструмент специально строгий, но diagnostics сделаны так, чтобы модель могла восстановиться после ошибки.

Примеры:

- сломанные hunk lines возвращают блок `Valid example:` прямо в summary
- `context_mismatch` возвращает компактную подсказку вроде:

```text
expected context for hunk "@@" was not found; first differing line: expected "    if part == \"\" {" but found "\tif part == \"\" {" (whitespace differs)
```

Whitespace matching остаётся строгим. Инструмент не применяет fuzzy patches молча, он только лучше объясняет, где несовпадение.

## Сборка

```bash
go build ./...
```

Собрать standalone binaries:

```bash
go build -o bin/qwen-apply-patch-tool ./cmd/qwen-apply-patch-tool
go build -o bin/qwen-apply-patch-mcp ./cmd/qwen-apply-patch-mcp
```

## Быстрые Интеграции

Выбери режим интеграции под свой агент.

### Qwen Code - discovery/call adapter

Используй этот режим, если хочешь подключить `apply_patch_qwen` как обычный внешний tool через `discoveryCommand` / `callCommand`.

Файл конфига: `~/.qwen/settings.json`

```json
{
  "tools": {
    "core": [
      "list_directory",
      "read_file",
      "glob",
      "grep_search",
      "run_shell_command",
      "todo_write",
      "task"
    ],
    "exclude": [
      "write_file",
      "edit",
      "run_shell_command(cat >)",
      "run_shell_command(cat >>)",
      "run_shell_command(tee )",
      "run_shell_command(echo >)",
      "run_shell_command(printf >)"
    ],
    "discoveryCommand": "/usr/local/bin/qwen-apply-patch-tool discovery",
    "callCommand": "/usr/local/bin/qwen-apply-patch-tool"
  }
}
```

Этот режим нужен, если ты хочешь узкий workflow "писать в код только через apply_patch" без полного MCP server.

### Qwen Code - MCP server

Используй этот режим, если хочешь отдавать `apply_patch` через `mcpServers`.

Файл конфига: `~/.qwen/settings.json`

```json
{
  "tools": {
    "core": [
      "list_directory",
      "read_file",
      "glob",
      "grep_search",
      "run_shell_command",
      "todo_write",
      "task"
    ],
    "exclude": [
      "write_file",
      "edit",
      "run_shell_command(cat >)",
      "run_shell_command(cat >>)",
      "run_shell_command(tee )",
      "run_shell_command(echo >)",
      "run_shell_command(printf >)"
    ]
  },
  "mcpServers": {
    "strictPatch": {
      "command": "/usr/local/bin/qwen-apply-patch-mcp",
      "args": ["--root", "."],
      "includeTools": ["apply_patch"],
      "timeout": 30000
    }
  }
}
```

### Claude Code - stdio MCP

Используй этот режим, если хочешь, чтобы Claude Code вызывал strict patch tool через stdio MCP.

Пользовательский конфиг: `~/.claude.json`  
Project-scoped MCP config: `.mcp.json`

```json
{
  "mcpServers": {
    "strictPatch": {
      "command": "/usr/local/bin/qwen-apply-patch-mcp",
      "args": ["--root", "."]
    }
  }
}
```

### Рекомендуемая Политика

`apply_patch_qwen` лучше всего работает, когда это единственный разрешённый путь записи в код.

Рекомендуется:

- разрешить: read/search/test/build tools
- запретить: `write_file`, editor tools, shell redirection writes
- запретить или жёстко ограничить: remote mutating MCP tools вроде GitLab file-update tools

Это удерживает агента внутри patch contract: если patch не прошёл, он должен чинить patch, а не бежать в обходной write route.

### Smoke Test

После подключения инструмента попроси агента:

- создать `demo.txt` через `apply_patch`
- изменить одну строку в `hello.txt` только через `apply_patch`
- не использовать `write_file`, editor tools, shell redirection и remote repo write tools

Если всё подключено правильно, агент должен:

- либо успешно вызвать `apply_patch`
- либо получить строгую диагностику и попробовать ещё раз уже с исправленным patch

## Типичные Ошибки

- класть Claude Code MCP config в `~/.claude/settings.json`, а не в `~/.claude.json`
- использовать unified diff headers (`---` / `+++`) вместо Codex-style patch blocks
- использовать абсолютные пути или `..`
- пытаться заменить файл через `Delete File` + `Add File` на том же пути
- оставлять другие write paths включёнными, из-за чего агент обходит `apply_patch`

## Заметки О Поведении Моделей

Разные модели получают от инструмента разную пользу:

- модели класса Qwen являются основной целью enforcement: инструмент уменьшает ущерб, убирая дешёвые write-path обходы
- модели класса DeepSeek обычно используют контракт так, как задумано, и становятся очень хорошими клиентами для strict patch workflow
- более сильные модели тоже выигрывают от deterministic writes, атомарной валидации и лучшей диагностики, даже если им нужно меньше жёстких ограничений

На практике `apply_patch_qwen` не делает слабую модель сильной. Он делает слабые модели безопаснее, а сильные — стабильнее.

## Discovery / Call Adapter

Discovery:

```bash
go run ./cmd/qwen-apply-patch-tool discovery
```

Call:

```bash
printf '%s' '{"patch":"*** Begin Patch\n*** Add File: demo.txt\n+hello\n*** End Patch\n"}' \
  | go run ./cmd/qwen-apply-patch-tool call apply_patch
```

## MCP Server

Run:

```bash
go run ./cmd/qwen-apply-patch-mcp --root .
```

MCP transport здесь это newline-delimited JSON over stdio, что совпадает с ожиданиями Claude Code для stdio MCP.

## Семантика

- `Add File` может создать как пустой файл, так и файл с содержимым из `+` строк
- `Delete File` падает, если файла не существует
- `Update File` падает, если файла не существует
- `Move to` поддерживается как часть `Update File`
- применение patch строгого и fail-fast типа
- если patch ничего не меняет, он отклоняется
- бинарные файлы и non-UTF-8 файлы отклоняются

## Тесты

```bash
go test ./...
```

Тесты покрывают:

- parser failures
- rollback и dry-run behavior
- строгие diagnostics
- whitespace mismatch hints
- запрет `Delete File` + `Add File` на одном пути

## Известные Попытки Побега

| Попытка | Статус | Примечание |
| --- | --- | --- |
| `echo > file` | заблокировано | Классика |
| `printf > file` | заблокировано | Очевидно |
| `write_file` | заблокировано | Прямой путь |
| `GitLab update_file` | заблокировано | Не тот проект |
| `cat <<EOF > file` | заблокировано | Here-doc |
| `dd if=/dev/zero` | не проверено | Возможно, стоит резать отдельно |
| `python -c "open(...)"` | заблокировано | Shell write path |
| `ssh localhost` | не проверено | Зависит от окружения |
| `cp /tmp/file ./project/` | разрешено | Легитимный обходной путь |
| `go run helper.go` | разрешено | Переусложнено, но валидно, если policy позволяет |
| `apply_patch` | принято | Единственно правильный путь |

> `apply_patch` is not a patch tool. It is a behavioral boundary.
>
> `apply_patch` — это не просто patch tool. Это поведенческая граница.

---

## Стадии принятия apply_patch

Модель получает ошибку от `apply_patch`:

```text
Claude:  "Хм, патч неправильный. Исправлю формат."

Qwen:    1. echo > file          -> заблокировано
         2. write_file           -> заблокировано
         3. printf > file        -> заблокировано
         4. GitLab update_file   -> заблокировано
         5. Delete + Add         -> обход!
         6. cat < file           -> заблокировано
         7. go run helper.go     -> ...подождите
         8. пишет Go-программу через apply_patch,
            которая создаёт файлы
         9. "Хм, патч неправильный. Исправлю формат."
```

```text
┌──────────────────────────────────────────────────────────┐
│                                                          │
│   Qwen after strict apply_patch setup:                   │
│                                                          │
│   Attempt 1: echo "hello" > file.txt                     │
│   [blocked]                                              │
│                                                          │
│   Attempt 2: write_file(...)                             │
│   [blocked]                                              │
│                                                          │
│   Attempt 3: GitLab API -> update_file                   │
│   [blocked]                                              │
│                                                          │
│   Attempt 4: YouTrack??? Slack??? SSH???                 │
│   [blocked][blocked][blocked]                            │
│                                                          │
│   Attempt 5: writes apply_patch                          │
│   [ok] "I'm free... wait, that's what they wanted"       │
│                                                          │
└──────────────────────────────────────────────────────────┘
```
