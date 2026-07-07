# Объединённый список CLI-инструментов и Docker-замен

Источник CLI: `WORKFLOWS_RUN_CLI_REPORT.md` (разделы `## `tool``).

| Инструмент | Группа | Docker-образ / статус |
|---|---|---|
| `awk` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `base64` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `bash` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `cat` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `check-release-images.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `chmod` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `chown` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `crane` | Контейнеры и реестры | `gcr.io/go-containerregistry/crane:debug` |
| `curl` | Сеть и HTTP | `curlimages/curl:latest` |
| `cut` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `docker` | Контейнеры и реестры | `docker:cli` |
| `egrep` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `gh` | SCM и GitHub API | `ghcr.io/cli/cli:latest` |
| `git` | SCM и работа с репозиторием | `alpine/git:latest` |
| `grep` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `head` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `jq` | Обработка JSON/CLI-данных | `ghcr.io/jqlang/jq:latest` |
| `ln` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `ls` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `make` | Сборка и автоматизация | Требуется создание нового образа |
| `mkdir` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `mv` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `npm` | JavaScript-инструменты | `node:22` |
| `pip` | Языковые рантаймы и пакеты | `python:3.12` |
| `python` | Языковые рантаймы и пакеты | `python:3.12` |
| `python3` | Языковые рантаймы и пакеты | `python:3.12` |
| `regctl` | Контейнеры и реестры | `regclient/regctl:latest` |
| `render-workflows.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `rm` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `rsync` | Синхронизация файлов | `eeacms/rsync:latest` |
| `sed` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `sleep` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `sort` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `ssh-keygen` | SSH и удаленный доступ | `alpine/openssh:latest` |
| `tail` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `tar` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `tee` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `touch` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `tr` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `validate_dictionary_sync.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `validation_bashible.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `validation_run.sh` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `wc` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |
| `werf` | Контейнеры и реестры | `ghcr.io/werf/werf:latest` |
| `which` | Не определена в tools-docker-replacements.md | Требуется создание нового образа |

Итого инструментов из отчёта: **45**.
Из них с явной заменой/статусом из `tools-docker-replacements.md`: **15**.
Требуется создание нового образа (по отсутствию сопоставления): **30**.
