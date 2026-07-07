# Объединённый список CLI-инструментов и Docker-замен

Источник CLI: `WORKFLOWS_RUN_CLI_REPORT.md` (разделы `## `tool``).

| Инструмент | Группа | Docker-образ / статус |
|---|---|---|
| `awk` | Не определена в tools-docker-replacements.md | часть busybox |
| `base64` | Не определена в tools-docker-replacements.md | часть busybox |
| `bash` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `cat` | Не определена в tools-docker-replacements.md | часть busybox |
| `check-release-images.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `chmod` | Не определена в tools-docker-replacements.md | часть busybox |
| `chown` | Не определена в tools-docker-replacements.md | часть busybox |
| `crane` | Контейнеры и реестры | `gcr.io/go-containerregistry/crane:debug` |
| `curl` | Сеть и HTTP | `curlimages/curl:latest` |
| `cut` | Не определена в tools-docker-replacements.md | часть busybox |
| `docker` | Контейнеры и реестры | `docker:cli` |
| `egrep` | Не определена в tools-docker-replacements.md | часть busybox |
| `gh` | SCM и GitHub API | `ghcr.io/cli/cli:latest` |
| `git` | SCM и работа с репозиторием | `alpine/git:latest` |
| `grep` | Не определена в tools-docker-replacements.md | часть busybox |
| `head` | Не определена в tools-docker-replacements.md | часть busybox |
| `jq` | Обработка JSON/CLI-данных | `ghcr.io/jqlang/jq:latest` |
| `ln` | Не определена в tools-docker-replacements.md | часть busybox |
| `ls` | Не определена в tools-docker-replacements.md | часть busybox |
| `make` | Сборка и автоматизация | Требуется создание нового образа |
| `mkdir` | Не определена в tools-docker-replacements.md | часть busybox |
| `mv` | Не определена в tools-docker-replacements.md | часть busybox |
| `npm` | JavaScript-инструменты | `node:22` |
| `pip` | Языковые рантаймы и пакеты | `python:3.12` |
| `python` | Языковые рантаймы и пакеты | `python:3.12` |
| `python3` | Языковые рантаймы и пакеты | `python:3.12` |
| `regctl` | Контейнеры и реестры | `regclient/regctl:latest` |
| `render-workflows.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `rm` | Не определена в tools-docker-replacements.md | часть busybox |
| `rsync` | Синхронизация файлов | `eeacms/rsync:latest` |
| `sed` | Не определена в tools-docker-replacements.md | часть busybox |
| `sleep` | Не определена в tools-docker-replacements.md | часть busybox |
| `sort` | Не определена в tools-docker-replacements.md | часть busybox |
| `ssh-keygen` | SSH и удаленный доступ | `alpine/openssh:latest` |
| `tail` | Не определена в tools-docker-replacements.md | часть busybox |
| `tar` | Не определена в tools-docker-replacements.md | часть busybox |
| `tee` | Не определена в tools-docker-replacements.md | часть busybox |
| `touch` | Не определена в tools-docker-replacements.md | часть busybox |
| `tr` | Не определена в tools-docker-replacements.md | часть busybox |
| `validate_dictionary_sync.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `validation_bashible.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `validation_run.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `wc` | Не определена в tools-docker-replacements.md | часть busybox |
| `werf` | Контейнеры и реестры | `ghcr.io/werf/werf:latest` |
| `which` | Не определена в tools-docker-replacements.md | часть busybox |

Итого инструментов из отчёта: **45**.
Из них с явной заменой/статусом из `tools-docker-replacements.md`: **15**.
Требуется создание нового образа (по отсутствию сопоставления): **30**.

## Дополнительно выявленные инструменты из совпавших `.sh` в `tools`

Источник анализа: `tools/check-release-images.sh`, `tools/docs/spelling/validate_dictionary_sync.sh`.

| Инструмент | Функция(и) / место обнаружения | Наличие в busybox |
|---|---|---|
| `diff` | основной поток скрипта (`validate_dictionary_sync.sh`) | Да |
| `dirname` | основной поток скрипта (`validate_dictionary_sync.sh`) | Да |
| `mktemp` | основной поток скрипта (`validate_dictionary_sync.sh`) | Да |
| `pwd` | основной поток скрипта (`validate_dictionary_sync.sh`) | Да |
| `validate_wordlist.sh` | основной поток скрипта (`validate_dictionary_sync.sh`) | Нет |

## Дополнительно выявленные инструменты из `validate_wordlist.sh`

Источник анализа: `tools/docs/spelling/validate_wordlist.sh`.

| Инструмент | Функция(и) / место обнаружения | Наличие в busybox |
|---|---|---|
| `uniq` | основной поток скрипта (`validate_wordlist.sh`), проверка дубликатов | Да |
