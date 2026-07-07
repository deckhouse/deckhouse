## CLI-инструменты, заменяемые Docker-образами

Исключены из рассмотрения как системные/базовые утилиты и артефакты парсинга: `chmod`, `chown`, `bash`, `rm`, `mkdir`, `ls`, `touch`, `cat`, `mv`, `egrep`, `sleep`, `tee`, `ln`, `tar`, `which`, `publish_image`, `pull_push_rmi`, `version`, `print`, `push_rmi`, `promote_with_att`, `regctl_copy`, `sign_current_minor`, `image`, `reproducibility`, `run`, `ls-remote`, `Dr.Web`, `Github`, `Kaspersky`, `max_patch`, `api`, `iter`, `iter++`, `keys`, `prev_minor`, `ref_name`.

| название инструмента | группа | докер образ которым можно заменить (если такого нет — укажи, что требуется создание нового) |
|---|---|---|
| `docker` | Контейнеры и реестры | `docker:cli` |
| `regctl` | Контейнеры и реестры | `regclient/regctl:latest` |
| `crane` | Контейнеры и реестры | `gcr.io/go-containerregistry/crane:debug` |
| `werf` | Контейнеры и реестры | `ghcr.io/werf/werf:latest` |
| `git` | SCM и работа с репозиторием | `alpine/git:latest` |
| `gh` | SCM и GitHub API | `ghcr.io/cli/cli:latest` |
| `curl` | Сеть и HTTP | `curlimages/curl:latest` |
| `ssh-keygen` | SSH и удаленный доступ | `alpine/openssh:latest` |
| `rsync` | Синхронизация файлов | `eeacms/rsync:latest` |
| `python` | Языковые рантаймы и пакеты | `python:3.12` |
| `python3` | Языковые рантаймы и пакеты | `python:3.12` |
| `pip` | Языковые рантаймы и пакеты | `python:3.12` |
| `npm` | JavaScript-инструменты | `node:22` |
| `jq` | Обработка JSON/CLI-данных | `ghcr.io/jqlang/jq:latest` |
| `make` | Сборка и автоматизация | Требуется создание нового образа |
| `slugify` | Текстовые утилиты | Требуется создание нового образа |
| `dhctl` | Deckhouse-специфичные инструменты | Требуется создание нового образа |
