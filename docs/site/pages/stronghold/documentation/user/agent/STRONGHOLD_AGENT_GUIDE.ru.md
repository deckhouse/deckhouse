---
title: "Stronghold агент"
permalink: ru/stronghold/documentation/agent/
lang: ru
---

## Содержание

- [Введение](#введение)
- [Сценарии использования](#сценарии-использования)
- [Основные возможности](#основные-возможности)
- [Настройка конфигурации](#настройка-конфигурации)
- [Запуск и управление](#запуск-и-управление)

## Введение

**Stronghold Agent** — это клиентский демон, который упрощает интеграцию приложений с Stronghold, предоставляя возможность автоматической аутентификации, управления токенами и доставки секретов без необходимости изменения кода приложения.

{% raw %}

### Зачем нужен Stronghold Agent?

Многие приложения, особенно legacy-системы, не имеют встроенной поддержки работы с системами управления секретами. 

Stronghold Agent решает эту проблему, действуя как посредник между приложением и Stronghold сервером:

- **Автоматическая аутентификация** — Agent самостоятельно проходит аутентификацию и обновляет токены
- **Доставка секретов** — Секреты доставляются в файлы или переменные окружения
- **Автоматическое обновление** — Секреты обновляются без перезапуска приложения (если приложение поддерживает повторное чтение конфигурационных файлов или ENV)
- **Упрощение интеграции** — Не требуется изменение кода приложения


![agent](/images/stronghold/agent.png)


## Сценарии использования

### 1. Развертывание на VM и Bare-Metal

**Основной сценарий использования Stronghold Agent** — это развертывание на виртуальных машинах (VM) и физических серверах (bare-metal). 
В отличие от Kubernetes, где есть нативные механизмы работы с секретами через CSI-драйверы и sidecar-контейнеры, на VM/bare-metal требуется отдельное решение.

**Преимущества:**
- Централизованное управление секретами для всего парка серверов
- Простая интеграция через systemd сервис
- Автоматическое обновление секретов без простоя
- Поддержка legacy систем без необходимости изменения кода приложений


### 2. Доставка секретов в Legacy приложения

Legacy приложения часто ожидают конфигурацию в виде:
- **Файлов конфигурации** (config.ini, application.properties, .env)
- **Переменных окружения** (ENV vars)
- **Файлов ключей и сертификатов**

Stronghold Agent может:
- Рендерить шаблоны с секретами в файлы
- Запускать приложения с инжектированными переменными окружения
- Автоматически перезапускать приложение при обновлении секретов

**Пример:** Spring Boot приложение получает credentials через переменные окружения.

Agent автоматически создает переменные окружения с секретами:

```bash
DB_USERNAME=v-approle-myapp-abc123
DB_PASSWORD=A1b2C3d4E5f6
DB_HOST=postgres.example.com
```

В `application.properties` просто используете эти переменные:

```java
spring.datasource.url=jdbc:postgresql://${DB_HOST}:5432/production
spring.datasource.username=${DB_USERNAME}
spring.datasource.password=${DB_PASSWORD}
```

Приложение не требует изменений - Spring Boot автоматически подставит значения из ENV.

### 3. Интеграция с приложениями без SDK поддержки

Многие приложения написаны на языках или фреймворках, для которых нет готовых SDK для работы со Stronghold:
- Legacy приложения C/C++
- Специализированные системы (SCADA, промышленное ПО)
- Бинарные приложения без исходного кода

Stronghold Agent позволяет таким приложениям использовать секреты через стандартные механизмы ОС.

### 4. Автоматическое обновление credentials без перезапуска

Stronghold Agent может отслеживать изменения секретов и:
- **Обновлять файлы** с новыми значениями
- **Отправлять сигналы** приложению (SIGHUP для перезагрузки конфигурации)
- **Выполнять команды** (скрипты перезагрузки, hot-reload)
- **Перезапускать процессы** при критических изменениях

**Пример:** Веб-сервер Nginx получает обновленные TLS сертификаты:

```hcl
template {
  source      = "/etc/nginx/ssl/cert.ctmpl"
  destination = "/etc/nginx/ssl/cert.pem"
  command     = "nginx -s reload"  # Перезагрузка без простоя
}
```

### 5. Получение динамических секретов

Stronghold поддерживает динамическую генерацию временных credentials для различных систем:

**Database credentials:**
- PostgreSQL, MySQL, MongoDB, Oracle
- Временные пользователи с ограниченным TTL
- Автоматическая ротация

**PKI сертификаты:**
- Автоматическая выдача TLS сертификатов
- Обновление перед истечением срока действия
- Поддержка различных CA

### 6. Использование в CI/CD пайплайнах

Для self-hosted CI/CD runners (Jenkins, GitLab Runner) Agent запускается на сервере runner'а и предоставляет секреты для пайплайнов.

**Как это работает:**
1. Agent устанавливается на сервер CI/CD runner
2. Аутентифицируется в Stronghold через AppRole
3. Рендерит секреты в файлы или предоставляет через API Proxy
4. Пайплайн читает секреты из локальных файлов

**Типичные сценарии:**
- **Деплой credentials** - SSH ключи, kubeconfig для развертывания
- **Registry access** - Docker registry credentials для pull/push образов
- **Cloud providers** - Облачные credentials для инфраструктуры


**Пример для GitLab Runner:**

```yaml
# .gitlab-ci.yml
deploy:
  script:
    - ssh -i /var/run/stronghold-agent/deploy_key deploy@server.example.com "cd /app && git pull && systemctl restart app"
```

**Роль Stronghold Agent:**

- Agent получает SSH ключ из Stronghold (например, из secret/data/ci/deploy_key)
- Рендерит его в файл /var/run/stronghold-agent/deploy_key с правами 0600
- При ротации ключа в Stronghold - agent автоматически обновляет файл
- GitLab Runner просто использует этот ключ - он всегда актуальный


## Основные возможности

### Templating (Рендеринг шаблонов)

Templating позволяет создавать файлы конфигурации, наполненные секретами из Stronghold, используя мощный язык шаблонов [Consul Template](https://github.com/hashicorp/consul-template) для рендеринга файлов. 


**Существует два режима “шаблонов”:**

1. **`template` (рендер в файл)** — Agent генерирует/обновляет файл на диске (например, `application.properties`, `nginx.conf`, `*.pem`) и при необходимости выполняет команду (`command`) для reload сервиса.
2. **`env_template` + `exec` (рендер в ENV и запуск процесса)** — Agent формирует значения переменных окружения и запускает приложение как дочерний процесс (`exec`). При изменении секретов процесс может быть перезапущен.


**Механизам работы Agent:**
1. Читает файл-шаблон с плейсхолдерами
2. Запрашивает секреты из Stronghold
3. Рендерит финальный файл, подставляя реальные значения.
4. Сохраняет файл с указанными правами доступа
5. (Опционально) Выполняет команду для перезагрузки приложения

**Когда использовать:**
- Legacy приложения, читающие конфиги из файлов
- Приложения без поддержки Stronghold/Vault API
- Необходимость доставки секретов в стандартные форматы (.properties, .conf, .ini, .yaml)
- Динамические database credentials
- PKI сертификаты

#### Синтаксис шаблонов

**Базовая структура:**

```go
{{ with secret "path/to/secret" }}
  {{ .Data.field_name }}
{{ end }}
```

**Для KV v2 (secret/data/...):**

```go
{{ with secret "secret/data/myapp" }}
username = {{ .Data.data.username }}
password = {{ .Data.data.password }}
{{ end }}
```

**Для динамических секретов (database, PKI):**

```go
{{ with secret "database/creds/myapp" }}
DB_USER={{ .Data.username }}
DB_PASS={{ .Data.password }}
{{ end }}
```

**Основные функции:**

| Функция | Описание | Пример |
|---------|----------|--------|
| `secret` | Получить секрет | `{{ with secret "secret/data/myapp" }}{{ .Data.data.password }}{{ end }}` |
| `base64Encode` | Кодирование в base64 | `{{ "password" \| base64Encode }}` |
| `base64Decode` | Декодирование из base64 | `{{ .Data.cert \| base64Decode }}` |
| `toJSON` | Преобразование в JSON | `{{ .Data \| toJSON }}` |
| `toYAML` | Преобразование в YAML | `{{ .Data \| toYAML }}` |
| `toLower` / `toUpper` | Изменение регистра | `{{ .Data.name \| toUpper }}` |
| `trim` | Удаление пробелов | `{{ .Data.value \| trim }}` |
| `range` | Цикл по массиву | `{{ range .Items }}{{ .Name }}{{ end }}` |
| `env` | Получить ENV переменную | `{{ env "HOME" }}` |
| `timestamp` | Текущее время | `{{ timestamp "2006-01-02 15:04:05" }}` |

#### Настройка Templating: Пошаговый пример

**Сценарий:** Legacy Java-приложение читает database credentials из `application.properties`

**Шаг 1: Сохранить секреты в Stronghold**

```bash
# Создать статический секрет
stronghold kv put secret/myapp/config \
  db_host=postgres.prod.example.com \
  db_port=5432 \
  db_name=production \
  db_user=app_user \
  db_password=SecureP@ssw0rd
```

**Шаг 2: Создать файл-шаблон**

Создаем `/etc/myapp/templates/application.properties.ctmpl`:

```text
# Database Configuration
{{ with secret "secret/data/myapp/config" }}
spring.datasource.url=jdbc:postgresql://{{ .Data.data.db_host }}:{{ .Data.data.db_port }}/{{ .Data.data.db_name }}
spring.datasource.username={{ .Data.data.db_user }}
spring.datasource.password={{ .Data.data.db_password }}
{{ end }}

# Подключение пула
spring.datasource.hikari.maximum-pool-size=10
spring.datasource.hikari.minimum-idle=5
```

**Шаг 3: Настроить Agent конфигурацию**

Создаем `/etc/stronghold-agent/agent.hcl`:

```hcl
# Подключение к Stronghold
stronghold {
  address = "https://stronghold.example.com:8200"
}

# Auto-Auth с AppRole
auto_auth {
  method {
    type = "approle"
    config = {
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
      remove_secret_id_file_after_reading = false
    }
  }
  
  sink {
    type = "file"
    config = {
      path = "/var/run/stronghold-agent/token"
    }
  }
}

# Templating блок
template {
  # Путь к шаблону
  source      = "/etc/myapp/templates/application.properties.ctmpl"
  
  # Путь к финальному файлу
  destination = "/etc/myapp/application.properties"
  
  # Права доступа (обязательно 0600 или 0400 для секретов)
  perms       = "0600"
  
  # Владелец файла (опционально)
  user        = "myapp"
  group       = "myapp"
  
  # Команда для перезагрузки приложения после изменения
  command     = "systemctl reload myapp"
  
  # Таймаут выполнения команды
  command_timeout = "30s"
  
  # Выполнять команду только при изменении содержимого
  # (не при каждом обновлении lease)
  wait {
    min = "2s"
    max = "10s"
  }
  
  # Ошибка если ключ отсутствует
  error_on_missing_key = true
}
```

**Шаг 4: Запустить Agent**

```bash
# Проверка конфигурации через пробный запуск
# Что делает: Agent читает конфиг, проходит аутентификацию, создает файл и сразу завершается
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -exit-after-auth -log-level=debug
```

**Что происходит при выполнении команды:**
1. Agent читает и парсит конфигурацию (`agent.hcl`)
2. Подключается к Stronghold серверу
3. Выполняет аутентификацию (AppRole: читает role-id и secret-id)
4. Получает токен и сохраняет в sink (`/var/run/stronghold-agent/token`)
5. Запрашивает секреты из Stronghold
6. Рендерит шаблон и создает файл (`/etc/myapp/application.properties`)
7. **Завершается с кодом 0** (успех) благодаря флагу `-exit-after-auth`


**Проверка успешности:**

```bash

# 1. Проверить что токен создан
ls -la /var/run/stronghold-agent/token
# Должен быть файл с недавней датой

# 2. Проверить что целевой файл создан
ls -la /etc/myapp/application.properties
# Должен быть файл с правами 0600

# 3. Проверить содержимое (осторожно - там пароли!)
sudo cat /etc/myapp/application.properties
# Должны быть реальные значения секретов, не {{ ... }}
```

**После успешной проверки - запуск как systemd service:**

```bash
systemctl start stronghold-agent
systemctl status stronghold-agent

# Проверка логов
journalctl -u stronghold-agent -f
```

#### Продвинутые сценарии

**Пример 1: Динамические database credentials**

```hcl
# Шаблон: /etc/myapp/db-config.conf.ctmpl
{{ with secret "database/creds/myapp-role" }}
# Auto-generated credentials (TTL: 1h)
# Rotation: automatic
DB_USER={{ .Data.username }}
DB_PASS={{ .Data.password }}
DB_LEASE_ID={{ .LeaseID }}
DB_LEASE_DURATION={{ .LeaseDuration }}
{{ end }}
```

Agent автоматически:
- Запрашивает временные credentials
- Обновляет файл при ротации (перед истечением TTL)
- Выполняет команду перезагрузки приложения

**Пример 2: PKI сертификаты**

```hcl
# Шаблон: /etc/nginx/ssl/cert.pem.ctmpl
{{ with secret "pki/issue/web-server" "common_name=app.example.com" "ttl=720h" }}
{{ .Data.certificate }}
{{ .Data.ca_chain }}
{{ end }}

# Шаблон: /etc/nginx/ssl/key.pem.ctmpl
{{ with secret "pki/issue/web-server" "common_name=app.example.com" "ttl=720h" }}
{{ .Data.private_key }}
{{ end }}
```

Agent конфигурация:

```hcl
template {
  source      = "/etc/nginx/ssl/cert.pem.ctmpl"
  destination = "/etc/nginx/ssl/cert.pem"
  perms       = "0644"
}

template {
  source      = "/etc/nginx/ssl/key.pem.ctmpl"
  destination = "/etc/nginx/ssl/key.pem"
  perms       = "0600"
  command     = "systemctl reload nginx"
}
```

**Пример 3: Условная логика и циклы**

```go
# Шаблон с условиями
{{ with secret "secret/data/myapp/config" }}
{{ if eq .Data.data.environment "production" }}
LOG_LEVEL=ERROR
DEBUG_MODE=false
{{ else }}
LOG_LEVEL=DEBUG
DEBUG_MODE=true
{{ end }}

API_KEY={{ .Data.data.api_key }}
{{ end }}

# Цикл по списку
{{ with secret "secret/data/myapp/allowed-ips" }}
{{ range $index, $ip := .Data.data.ips }}
allow {{ $ip }};
{{ end }}
{{ end }}
```

**Пример 4: Множественные секреты в одном файле**

```go
# Database credentials
{{ with secret "database/creds/app" }}
DB_USER={{ .Data.username }}
DB_PASS={{ .Data.password }}
{{ end }}

# API Keys
{{ with secret "secret/data/myapp/api-keys" }}
STRIPE_KEY={{ .Data.data.stripe_key }}
SENDGRID_KEY={{ .Data.data.sendgrid_key }}
{{ end }}

# Redis credentials
{{ with secret "secret/data/myapp/redis" }}
REDIS_HOST={{ .Data.data.host }}
REDIS_PASSWORD={{ .Data.data.password }}
{{ end }}
```

#### Важные параметры template блока

| Параметр | Описание | Пример |
|----------|----------|--------|
| `source` | Путь к файлу-шаблону | `/etc/app/template.ctmpl` |
| `destination` | Путь к результирующему файлу | `/etc/app/config.conf` |
| `perms` | Права доступа (восьмеричная система) | `"0600"`, `"0644"` |
| `user` | Владелец файла | `"myapp"` |
| `group` | Группа файла | `"myapp"` |
| `command` | Команда после рендеринга | `"systemctl reload app"` |
| `command_timeout` | Таймаут команды | `"30s"` |
| `error_on_missing_key` | Ошибка при отсутствии ключа | `true` / `false` |
| `wait.min` | Минимальное время между обновлениями | `"2s"` |
| `wait.max` | Максимальное время между обновлениями | `"10s"` |
| `backup` | Создавать backup перед перезаписью | `true` / `false` |

#### `template` vs `env_template` — в чём разница и что выбрать

**`template`** — это рендеринг секретов **в файл на диске**.

- Используйте `template`, если приложение/сервис читает конфигурацию из файлов: `.conf/.ini/.yaml/.properties`, TLS `*.pem`, ключи, сертификаты и т.п.
- В `template` доступен полный “файловый” набор опций: `destination`, `perms`, `user/group`, `backup`, `wait`, а также `command` для перезагрузки сервиса после изменения (например `systemctl reload nginx`).

**`env_template` + `exec`** — это запуск приложения как дочернего процесса, где секреты попадают **в переменные окружения процесса**.

- Используйте `env_template`, если приложение читает конфигурацию из ENV (12-factor style) и допустим перезапуск при ротации секретов.
- В Stronghold Agent каждый `env_template` задаёт значение **ровно одной** переменной окружения и всегда пишется как `env_template "VAR_NAME" { ... }`.
- Важно: `env_template` **не** создаёт `.env` файл. Поля `destination/perms/command/wait/...` для `env_template` не поддерживаются (они доступны только в `template`).

**Практические нюансы:**

- Если вы используете `template` и запускаете Agent под systemd с hardening (`ProtectSystem=strict`), убедитесь что `ReadWritePaths` включает директорию `template.destination` (иначе рендеринг будет падать из-за запрета записи).
- Если вы используете `env_template` для запуска Docker-контейнера, переменные окружения нужно явно пробрасывать в `docker run` через `--env VAR_NAME` (или другим способом), иначе они останутся только в окружении самого Agent.


### Режим супервизора процессов (Process Supervisor Mode)

Режим Process Supervisor позволяет Agent запускать приложение как дочерний процесс и инжектировать секреты напрямую в переменные окружения.

**Концепция:**

В этом режиме Agent:
1. Запускается как родительский процесс
2. Запрашивает секреты из Stronghold
3. Формирует переменные окружения из шаблона
4. Запускает приложение как дочерний процесс с этими переменными
5. Мониторит изменения секретов
6. При изменении секретов - перезапускает приложение с новыми значениями

**Ограничения режима (важно):**
- `exec` должен использоваться вместе хотя бы с одним `env_template`.
- `env_template` **нельзя** комбинировать с `template` и `api_proxy` в одном конфиге (это разные режимы работы).
- `env_template` всегда задается как `env_template "VAR_NAME" { ... }` и формирует значение ровно одной переменной окружения.

**Преимущества:**
- Секреты **никогда не записываются на диск**
- Автоматический перезапуск при обновлении секретов
- Изоляция секретов на уровне процесса
- Подходит для 12-factor приложений
- Простая миграция legacy-приложений на ENV-конфиги

**Когда использовать:**
- Приложения, читающие конфигурацию из ENV переменных
- Высокие требования к безопасности (секреты не должны касаться диска)
- Контейнеризированные приложения на VM
- Динамические credentials с частой ротацией
- Разработка и тестирование

#### Настройка Process Supervisor: Пошаговый пример

**Сценарий:** Java Spring Boot приложение читает секреты из переменных окружения

**Шаг 1: Подготовить приложение**

Spring Boot приложение должно читать конфигурацию из ENV:

**Конфигурация application.properties:**

```
# application.properties - использует ENV переменные
server.port=8080

# Database - значения берутся из ENV
spring.datasource.url=${DB_URL}
spring.datasource.username=${DB_USERNAME}
spring.datasource.password=${DB_PASSWORD}
spring.datasource.driver-class-name=org.postgresql.Driver

# JPA
spring.jpa.hibernate.ddl-auto=validate
spring.jpa.properties.hibernate.dialect=org.hibernate.dialect.PostgreSQLDialect

# API Key
api.key=${API_KEY}
```

**Шаг 2: Сохранить секреты в Stronghold**

```bash
# Настроить Database Secrets Engine для динамических credentials
stronghold write database/config/postgresql \
  plugin_name=postgresql-database-plugin \
  allowed_roles="myapp-role" \
  connection_url="postgresql://{{username}}:{{password}}@postgres.prod:5432/myapp?sslmode=require" \
  username="vault_admin" \
  password="admin_password"

# Создать роль для приложения
stronghold write database/roles/myapp-role \
  db_name=postgresql \
  creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; \
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO \"{{name}}\";" \
  default_ttl="1h" \
  max_ttl="24h"

# Статические секреты (API ключи)
stronghold kv put secret/myapp/config \
  api_key=sk_live_1234567890abcdef
```

**Шаг 3: Настроить Agent в режиме Supervisor**

Создаем `/etc/stronghold-agent/agent.hcl`:

```hcl
# Подключение к Stronghold
stronghold {
  address = "https://stronghold.example.com:8200"
}

# Auto-Auth с AppRole
auto_auth {
  method {
    type = "approle"
    config = {
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
      remove_secret_id_file_after_reading = false
    }
  }
}

# Process Supervisor - запуск Spring Boot
exec {
  # Команда для запуска приложения
  command = ["/usr/bin/java", "-jar", "/opt/myapp/demo-application.jar"]
  
  # Перезапуск при изменении секретов (ротация database credentials)
  restart_on_secret_changes = "always"
  
  # Сигнал для остановки процесса (по умолчанию SIGTERM)
  restart_stop_signal = "SIGTERM"
}

# Шаблон переменных окружения
# Важно: `env_template` задается в виде `env_template "ИМЯ_ПЕРЕМЕННОЙ" { ... }`.
# Каждый блок формирует значение одной ENV переменной, которая будет передана дочернему процессу.
env_template "DB_URL" {
  contents = "jdbc:postgresql://postgres.prod:5432/myapp"
}

env_template "DB_USERNAME" {
  contents = "{{ with secret \"database/creds/myapp-role\" }}{{ .Data.username }}{{ end }}"
}

env_template "DB_PASSWORD" {
  contents = "{{ with secret \"database/creds/myapp-role\" }}{{ .Data.password }}{{ end }}"
}

env_template "API_KEY" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.api_key }}{{ end }}"
}

env_template "JAVA_OPTS" {
  contents = "-Xmx2g -Xms512m -XX:+UseG1GC"
}

env_template "SPRING_PROFILES_ACTIVE" {
  contents = "production"
}
```

**Шаг 4: Запустить Agent**

```bash
# Agent запустит Java приложение с секретами в ENV
stronghold-agent -config=/etc/stronghold-agent/agent.hcl
```

Agent автоматически:
- Аутентифицируется в Stronghold
- Получает database credentials и API ключи
- Запускает Java приложение с секретами в переменных окружения
- При ротации credentials - перезапускает приложение с новыми значениями

#### Примеры для разных языков программирования

**Go приложение:**

```hcl
exec {
  command = ["/opt/myapp/myapp-server"]
  restart_on_secret_changes = "always"
  restart_stop_signal = "SIGTERM"
}

env_template "DB_HOST" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.db_host }}{{ end }}"
}
env_template "DB_PORT" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.db_port }}{{ end }}"
}
env_template "DB_NAME" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.db_name }}{{ end }}"
}
env_template "DB_USER" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.db_user }}{{ end }}"
}
env_template "DB_PASSWORD" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.db_password }}{{ end }}"
}
env_template "API_KEY" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.api_key }}{{ end }}"
}
env_template "LOG_LEVEL" {
  contents = "info"
}
```

**Docker контейнер на VM:**

```hcl
exec {
  command = [
    "/usr/bin/docker", "run", "--rm",
    "--name", "myapp",
    "-p", "8080:8080",
    # Пробросить переменные окружения из окружения Stronghold Agent внутрь контейнера:
    "--env", "DOCKER_ENV_API_KEY",
    "--env", "DOCKER_ENV_DATABASE_URL",
    "myapp:latest"
  ]
  restart_on_secret_changes = "always"
  restart_stop_signal = "SIGTERM"
}

env_template "DOCKER_ENV_API_KEY" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.api_key }}{{ end }}"
}

env_template "DOCKER_ENV_DATABASE_URL" {
  contents = "{{ with secret \"secret/data/myapp/config\" }}{{ .Data.data.database_url }}{{ end }}"
}
```

#### Важные параметры exec блока

| Параметр | Описание | Значение по умолчанию |
|----------|----------|-----------------------|
| `command` | Команда запуска приложения (массив) | - (обязательный) |
| `restart_on_secret_changes` | Перезапуск при изменении секретов: `never`, `always` | `always` |
| `restart_stop_signal` | Сигнал для остановки процесса | `SIGTERM` |

#### Важные параметры env_template блока

| Параметр | Описание | Пример |
|----------|----------|--------|
| `contents` | Inline шаблон переменных окружения | `<<-EOT ... EOT` |
| `source` | Путь к файлу-шаблону (альтернатива `contents`) | `"/etc/app/env.ctmpl"` |
| `error_on_missing_key` | Ошибка при отсутствии ключа | `true` / `false` |

> Примечание: в Stronghold Agent блок `env_template` всегда имеет имя переменной окружения: `env_template "MY_VAR" { ... }`.
> Поля `destination/perms/command/wait/...` в `env_template` **не поддерживаются** (они доступны только в обычном `template` блоке).

#### Управление жизненным циклом процесса

**Автоматический перезапуск:**

При изменении секретов (например, ротация database credentials) Agent:
1. Получает новые секреты
2. Формирует новые ENV переменные
3. Отправляет `SIGTERM` дочернему процессу (приложение должно корректно обрабатывать `SIGTERM`).
4. Перезапускает процесс с обновленными переменными


### Кэширование и ротация токенов

Agent автоматически управляет жизненным циклом токенов:

**Кэширование токена (Token Caching):**
- Кеширование токена после аутентификации
- Использование кешированного токена для всех запросов
- Уменьшение нагрузки на Stronghold сервер

**Автоматическое обновление токена (Token Renewal):**

Agent получает токен с ограниченным сроком действия (например, 1 час) и заранее продлевает его. Если продление невозможно — Agent переаутентифицируется.

**Зачем:** Чтобы приложение могло работать непрерывно, без ручного вмешательства.

**Автоматическое обновление секретов (Lease Renewal):**

Динамические секреты (например, database credentials) тоже имеют срок действия. Agent автоматически продлевает их перед истечением, затем обновляет файлы конфигурации и может перезагрузить приложение.

**Зачем:** Credentials всегда актуальны, приложение не падает из-за истекших паролей.

### API Proxy

Agent может выступать в роли прокси для Stronghold API:

**Возможности:**
- Локальный HTTP(S) endpoint для приложений
- Автоматическое добавление токена аутентификации
- Кеширование ответов (опционально)
- Снижение сетевой нагрузки

**Конфигурация:**

```hcl
api_proxy {
  use_auto_auth_token = true
}

listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
}
```

**Использование приложением:**

```bash
# Приложение обращается к локальному Agent
curl http://127.0.0.1:8200/v1/secret/data/myapp

# Agent автоматически добавляет токен и проксирует на Stronghold сервер
```

### Auto-Auth (Автоматическая аутентификация)

Auto-Auth — это ключевая возможность Stronghold Agent, которая полностью автоматизирует процесс получения и обновления токена аутентификации.

**Как это работает:**
1. Agent запускается с настроенным методом аутентификации
2. Автоматически проходит аутентификацию в Stronghold
3. Получает токен и использует его для своих операций (templating, API proxy)
4. Если настроен sink — записывает токен в файл для использования другими процессами
5. Автоматически обновляет токен перед истечением TTL
6. При необходимости переаутентифицируется

**Что такое sink:**
Sink  — это файл, куда Agent записывает полученный токен. Настройка sink опциональна:
- **Если sink настроен:** токен записывается в файл (например, `/var/run/stronghold-agent/token`), который могут читать другие процессы
- **Если sink не настроен:** токен используется только внутри Agent для его собственных операций (templating, caching)

**Поддерживаемые методы аутентификации:**
- **AppRole** — рекомендуемый для VM/bare-metal
- **Token** — для простых сценариев
- **JWT/OIDC** — для интеграции с identity providers
- **Облачные провайдеры**


### AppRole (Рекомендуемый для VM/Bare-Metal)

AppRole — это метод аутентификации, предназначенный для машин и приложений.

**Концепция:**
- **Role ID** — идентификатор роли (аналог username)
- **Secret ID** — секретный идентификатор (аналог password)
- Оба ID требуются для аутентификации

**Преимущества:**
- Разделение обязанностей (Role ID и Secret ID доставляются разными путями)
- Гибкая настройка политик
- Поддержка CIDR restrictions
- Secret ID может быть одноразовым

**Настройка на Stronghold сервере:**

```bash
# Включить AppRole
stronghold auth enable approle

# Создать роль
stronghold write auth/approle/role/myapp \
  token_ttl=1h \
  token_max_ttl=4h \
  policies="myapp-policy"

# Получить Role ID
stronghold read auth/approle/role/myapp/role-id

# Создать Secret ID
stronghold write -f auth/approle/role/myapp/secret-id
```

**Конфигурация в Agent:**

```hcl
auto_auth {
  method "approle" {
    mount_path = "auth/approle"
    config = {
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
      remove_secret_id_file_after_reading = true
    }
  }
}
```

**Доставка и хранение credentials:**

**Role ID:**
- **Что это:** Публичный идентификатор роли, не является секретом
- **Как доставляется:** Через configuration management (Ansible, Puppet), в образе VM, или вручную
- **Где хранится:** Путь задается в конфигурации Agent параметром `role_id_file_path`
  - Типичный путь: `/etc/stronghold-agent/role-id` 
  - Права доступа: 0640, owner: stronghold-agent
  - Можно использовать любой путь по вашему выбору
- **Удаление:** НЕ удаляется после использования, используется повторно при переаутентификации
- **Безопасность:** Можно хранить в git репозитории, не критично если будет скомпрометирован (без Secret ID бесполезен)

**Secret ID:**
- **Что это:** Секретный идентификатор, аналог пароля
- **Как доставляется:** 
  - Вручную администратором при первом запуске сервера
  - Через защищенное SSH соединение
  - Через внутренний портал самообслуживания (если есть)
  - Через encrypted переменную в CI/CD системе
  - НЕ должен быть в git или configuration management
- **Где хранится:** Путь задается в конфигурации Agent параметром `secret_id_file_path`
  - Типичный путь: `/etc/stronghold-agent/secret-id`
  - Права доступа: 0640, owner: stronghold-agent
  - Можно использовать любой путь по вашему выбору
- **Удаление:** Может быть удален после использования (параметр `remove_secret_id_file_after_reading = true` в конфиге Agent)
- **Безопасность:** КРИТИЧНЫЙ секрет, должен быть защищен

**Типы Secret ID:**

 Параметры `secret_id_num_uses` и `secret_id_ttl` задаются на Stronghold сервере при создании роли или генерации Secret ID. Agent просто использует уже созданный Secret ID.

1. **Одноразовый (num_uses=1):**
   ```bash
   # На Stronghold сервере при создании роли:
   stronghold write auth/approle/role/myapp \
     secret_id_num_uses=1 \
     policies="myapp-policy"
   
   # Или при генерации конкретного Secret ID:
   stronghold write -f auth/approle/role/myapp/secret-id num_uses=1
   ```
   - Используется только один раз для аутентификации
   - После использования становится невалидным
   - Наиболее безопасный вариант для production
   - Agent должен иметь `remove_secret_id_file_after_reading = true` в конфиге

2. **Многоразовый (num_uses=0):**
   ```bash
   # На Stronghold сервере:
   stronghold write auth/approle/role/myapp \
     secret_id_num_uses=0 \
     policies="myapp-policy"
   ```
   - Может использоваться множество раз
   - Удобен для тестирования и разработки
   - Менее безопасен (если скомпрометирован - требуется ручная ротация)

3. **С ограниченным TTL:**
   ```bash
   # На Stronghold сервере:
   stronghold write auth/approle/role/myapp \
     secret_id_ttl=24h \
     policies="myapp-policy"
   ```
   - Истекает через указанное время (24 часа в примере)
   - Баланс между безопасностью и удобством
   - После истечения TTL требуется новый Secret ID

**Полный пример настройки и доставки credentials:**

```bash
# ============================================================================
# ШАГ 1: Настройка на Stronghold сервере (выполняет администратор)
# ============================================================================

# Включить AppRole метод аутентификации
stronghold auth enable approle

# Создать политику доступа для приложения
stronghold policy write myapp-policy - <<EOF
path "secret/data/myapp/*" {
  capabilities = ["read"]
}
path "database/creds/myapp" {
  capabilities = ["read"]
}
EOF

# Создать роль AppRole с настройками
stronghold write auth/approle/role/myapp \
  token_ttl=1h \                    # Время жизни токена (обновляется автоматически)
  token_max_ttl=4h \                # Максимальное время жизни токена (после этого требуется переаутентификация)
  policies="myapp-policy" \         # Политика доступа (какие секреты может читать)
  secret_id_num_uses=1 \            # Количество использований Secret ID (1 = одноразовый)
  secret_id_ttl=24h                 # Время жизни Secret ID (истекает через 24 часа)

# Получить Role ID
stronghold read auth/approle/role/myapp/role-id
# Вывод: role_id    abc123-def456-ghi789

# Сгенерировать одноразовый Secret ID
stronghold write -f auth/approle/role/myapp/secret-id
# Вывод: secret_id    xyz789-abc123-def456

# ============================================================================
# ШАГ 2: Доставка credentials на целевой сервер
# ============================================================================

# Создать директорию на целевом сервере
ssh root@app-server.example.com << 'ENDSSH'
  mkdir -p /etc/stronghold-agent
  chown root:stronghold-agent /etc/stronghold-agent
  chmod 750 /etc/stronghold-agent
ENDSSH

# Доставить Role ID (можно через automation или вручную)
ssh root@app-server.example.com << 'ENDSSH'
  echo -n "abc123-def456-ghi789" > /etc/stronghold-agent/role-id
  chown stronghold-agent:stronghold-agent /etc/stronghold-agent/role-id
  chmod 0640 /etc/stronghold-agent/role-id
ENDSSH

# Доставить Secret ID через защищенный канал (одноразовый)
ssh root@app-server.example.com << 'ENDSSH'
  echo -n "xyz789-abc123-def456" > /etc/stronghold-agent/secret-id
  chown stronghold-agent:stronghold-agent /etc/stronghold-agent/secret-id
  chmod 0640 /etc/stronghold-agent/secret-id
ENDSSH

# ============================================================================
# ШАГ 3: Настройка конфигурации Agent на целевом сервере
# ============================================================================

# Создать конфигурационный файл
cat > /etc/stronghold-agent/agent.hcl <<EOF
stronghold {
  address = "https://stronghold.example.com:8200"
}

auto_auth {
  method "approle" {
    mount_path = "auth/approle"
    config = {
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
      remove_secret_id_file_after_reading = true
    }
  }
  
  sink "file" {
    config = {
      path = "/var/run/stronghold-agent/token"
      mode = 0640
    }
  }
}
EOF

# Установить права на конфигурацию
chown root:stronghold-agent /etc/stronghold-agent/agent.hcl
chmod 0640 /etc/stronghold-agent/agent.hcl

# ============================================================================
# ШАГ 4: Запуск Agent
# ============================================================================

# Запустить Agent
systemctl start stronghold-agent

# Проверить успешную аутентификацию
journalctl -u stronghold-agent -n 50 | grep -i "authentication successful"

# Проверить наличие токена
ls -la /var/run/stronghold-agent/token

# Проверить, что Secret ID удален (если remove_secret_id_file_after_reading = true)
ls -la /etc/stronghold-agent/secret-id
# Должно быть: No such file or directory

# ============================================================================
# ШАГ 5: Последующая работа
# ============================================================================

# При перезапуске Agent будет использовать только Role ID
# Токен обновляется автоматически каждые ~59 минут (за 1 минуту до истечения TTL)
# Secret ID больше не требуется
```

**Best Practices:**
- Использовать одноразовые Secret ID для production
- Разделять доставку: Role ID через automation, Secret ID через защищенный канал
- Ограничивать доступ по CIDR (`secret_id_bound_cidrs`)
- Логировать все использования Secret ID в Stronghold для аудита

### Token (Для простых сценариев)

Прямое использование токена — самый простой метод аутентификации. Agent читает готовый токен из файла и использует его.

**Концепция:**
- Токен создается заранее администратором на Stronghold сервере
- Доставляется на целевой сервер любым удобным способом
- Agent просто читает токен из файла и использует

**Когда использовать:**
- Тестовые окружения и разработка
- Временные инсталляции
- Сценарии, где нельзя использовать AppRole
- Простые cases без высоких требований к безопасности

**Недостатки:**
- Менее безопасно (токен — это долгоживущий credential)
- Нет разделения обязанностей (как в AppRole)
- При компрометации требуется ручная ротация
- Не рекомендуется для production окружений

**Полный пример настройки:**

```bash
# ============================================================================
# ШАГ 1: Создание токена на Stronghold сервере
# ============================================================================

# Создать политику доступа
stronghold policy write myapp-policy - <<EOF
path "secret/data/myapp/*" {
  capabilities = ["read"]
}
path "database/creds/myapp" {
  capabilities = ["read"]
}
EOF

# Создать токен с параметрами
stronghold token create \
  -policy=myapp-policy \
  -ttl=720h \                  # Время жизни 30 дней
  -renewable=true \            # Можно обновлять
  -display-name="myapp-agent" \
  -format=json

# Вывод:
# {
#   "auth": {
#     "client_token": "hvs.CAES...xyz123",
#     "policies": ["default", "myapp-policy"],
#     "renewable": true,
#     "lease_duration": 2592000
#   }
# }

# Сохранить токен
export AGENT_TOKEN="hvs.CAES...xyz123"

# ============================================================================
# ШАГ 2: Доставка токена на целевой сервер
# ============================================================================

# Создать директорию
ssh root@app-server.example.com << 'ENDSSH'
  mkdir -p /etc/stronghold-agent
  chown root:stronghold-agent /etc/stronghold-agent
  chmod 750 /etc/stronghold-agent
ENDSSH

# Доставить токен через защищенный канал
echo -n "$AGENT_TOKEN" | ssh root@app-server.example.com 'cat > /etc/stronghold-agent/token'
ssh root@app-server.example.com << 'ENDSSH'
  chown stronghold-agent:stronghold-agent /etc/stronghold-agent/token
  chmod 0640 /etc/stronghold-agent/token
ENDSSH

# ============================================================================
# ШАГ 3: Настройка конфигурации Agent
# ============================================================================

cat > /etc/stronghold-agent/agent.hcl <<EOF
stronghold {
  address = "https://stronghold.example.com:8200"
}

auto_auth {
  method "token_file" {
    config = {
      token_file_path = "/etc/stronghold-agent/token"
    }
  }
  
  # Sink опционален для token метода
  sink "file" {
    config = {
      path = "/var/run/stronghold-agent/token"
      mode = 0640
    }
  }
}

# Пример шаблона
template {
  source = "/etc/stronghold-agent/templates/database.conf.ctmpl"
  destination = "/etc/myapp/database.conf"
  perms = "0600"
}
EOF

chown root:stronghold-agent /etc/stronghold-agent/agent.hcl
chmod 0640 /etc/stronghold-agent/agent.hcl

# ============================================================================
# ШАГ 4: Запуск Agent
# ============================================================================

systemctl start stronghold-agent

# Проверить работу
journalctl -u stronghold-agent -n 50

# Токен будет автоматически обновляться до истечения max_ttl
```

**Важные параметры токена:**

- **ttl** — начальное время жизни токена
- **renewable** — можно ли обновлять токен (должен быть true для Agent)
- **period** — если задан, токен обновляется на этот период (например, `period=24h` — токен обновляется каждые 24 часа)
- **explicit-max-ttl** — абсолютное максимальное время жизни (после этого токен перестает обновляться)

**Рекомендации по безопасности:**

1. Использовать токены с `renewable=true` для автоматического обновления
2. Установить разумный TTL (например, 30 дней)
3. Настроить explicit-max-ttl для ограничения общего времени жизни
4. Регулярно проверять и отзывать неиспользуемые токены
5. Хранить токен с минимальными правами доступа (0640)
6. Для production рассмотреть использование AppRole вместо token

### JWT/OIDC (Для интеграции с Identity Providers)

JWT/OIDC аутентификация позволяет использовать существующую инфраструктуру identity management для аутентификации в Stronghold.

**Концепция:**
- Приложение получает JWT токен от Identity Provider (Keycloak, Azure AD, Google, etc.)
- JWT токен содержит claims (утверждения) о пользователе или сервисе
- Stronghold проверяет подпись JWT и извлекает claims
- На основе claims выдается Stronghold токен с соответствующими политиками

**Сценарии использования:**
- Интеграция с корпоративным SSO (Single Sign-On)
- Использование service accounts из Identity Provider
- Федеративная аутентификация между организациями
- CI/CD интеграция через OIDC (GitHub Actions, GitLab CI)

**Преимущества:**
- Централизованное управление идентификацией
- Не нужно создавать отдельные credentials для каждого приложения
- Автоматическая ротация JWT токенов через Identity Provider
- Поддержка MFA и других возможностей IdP

**Полный пример настройки (с Keycloak):**

```bash
# ============================================================================
# ШАГ 1: Настройка JWT auth метода на Stronghold сервере
# ============================================================================

# Включить JWT auth метод
stronghold auth enable jwt

# Настроить JWT метод с параметрами Keycloak
stronghold write auth/jwt/config \
  oidc_discovery_url="https://keycloak.example.com/realms/myrealm" \
  oidc_client_id="stronghold" \
  oidc_client_secret="client-secret-from-keycloak" \
  default_role="default"

# Создать политику доступа
stronghold policy write myapp-jwt-policy - <<EOF
path "secret/data/myapp/*" {
  capabilities = ["read"]
}
path "database/creds/myapp" {
  capabilities = ["read"]
}
EOF

# Создать роль для JWT аутентификации
stronghold write auth/jwt/role/myapp-role \
  role_type="jwt" \
  bound_audiences="stronghold" \
  user_claim="sub" \
  bound_subject="service-account-myapp" \
  token_ttl=1h \
  token_max_ttl=4h \
  token_policies="myapp-jwt-policy"

# Параметры роли:
# - bound_audiences: какие audience должны быть в JWT
# - user_claim: какой claim использовать как username
# - bound_subject: конкретное значение subject (опционально)
# - bound_claims: дополнительные требования к claims

# Пример с более сложными условиями:
stronghold write auth/jwt/role/myapp-role \
  role_type="jwt" \
  bound_audiences="stronghold" \
  user_claim="sub" \
  bound_claims='{"environment":"production","app":"myapp"}' \
  claim_mappings='{"department":"dept"}' \
  token_policies="myapp-jwt-policy"

# ============================================================================
# ШАГ 2: Получение JWT токена от Identity Provider
# ============================================================================

# Пример 1: Keycloak service account
curl -X POST "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=myapp-service" \
  -d "client_secret=service-secret" \
  -d "grant_type=client_credentials" \
  | jq -r '.access_token' > /tmp/jwt-token.txt

# Пример 2: GitHub Actions OIDC token
# В GitHub Actions workflow:
# - uses: actions/checkout@v3
# - name: Get OIDC token
#   run: |
#     curl -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
#          "$ACTIONS_ID_TOKEN_REQUEST_URL" | jq -r '.value' > jwt-token.txt

# ============================================================================
# ШАГ 3: Доставка JWT токена на целевой сервер
# ============================================================================

# Создать директорию
ssh root@app-server.example.com << 'ENDSSH'
  mkdir -p /etc/stronghold-agent
  chown root:stronghold-agent /etc/stronghold-agent
  chmod 750 /etc/stronghold-agent
ENDSSH

# Доставить JWT токен
scp /tmp/jwt-token.txt root@app-server.example.com:/etc/stronghold-agent/jwt-token
ssh root@app-server.example.com << 'ENDSSH'
  chown stronghold-agent:stronghold-agent /etc/stronghold-agent/jwt-token
  chmod 0640 /etc/stronghold-agent/jwt-token
ENDSSH

# ============================================================================
# ШАГ 4: Настройка конфигурации Agent
# ============================================================================

cat > /etc/stronghold-agent/agent.hcl <<EOF
stronghold {
  address = "https://stronghold.example.com:8200"
}

auto_auth {
  method "jwt" {
    mount_path = "auth/jwt"
    config = {
      path = "/etc/stronghold-agent/jwt-token"
      role = "myapp-role"
    }
  }
  
  sink "file" {
    config = {
      path = "/var/run/stronghold-agent/token"
      mode = 0640
    }
  }
}

template {
  source = "/etc/stronghold-agent/templates/database.conf.ctmpl"
  destination = "/etc/myapp/database.conf"
  perms = "0600"
}
EOF

chown root:stronghold-agent /etc/stronghold-agent/agent.hcl
chmod 0640 /etc/stronghold-agent/agent.hcl

# ============================================================================
# ШАГ 5: Запуск Agent
# ============================================================================

systemctl start stronghold-agent

# Проверить успешную аутентификацию
journalctl -u stronghold-agent -n 50 | grep -i "authentication successful"

# Проверить Stronghold токен
cat /var/run/stronghold-agent/token
```

**Особенности JWT метода:**

1. **JWT токен vs Stronghold токен:**
   - JWT токен — это credential от Identity Provider (короткоживущий, обычно 5-60 минут)
   - После аутентификации Agent получает Stronghold токен
   - Stronghold токен обновляется автоматически (как в AppRole)
   - JWT токен НЕ обновляется Agent автоматически

2. **Периодическое обновление JWT:**
   - Если JWT истекает, нужно получить новый от IdP
   - Можно настроить cron job для периодического обновления:
   ```bash
   # /etc/cron.d/refresh-jwt
   */30 * * * * stronghold-agent /usr/local/bin/refresh-jwt-token.sh
   ```

**Проверка JWT токена:**

```bash
# Декодировать JWT для проверки claims
cat /etc/stronghold-agent/jwt-token | cut -d. -f2 | base64 -d | jq

# Вывод покажет claims:
# {
#   "sub": "service-account-myapp",
#   "aud": "stronghold",
#   "iss": "https://keycloak.example.com/realms/myrealm",
#   "exp": 1234567890,
#   "iat": 1234567800
# }
```

**Рекомендации:**

1. Использовать короткий TTL для JWT токенов (5-15 минут)
2. Настроить bound_audiences для защиты от переиспользования токенов
3. Использовать bound_subject или bound_claims для строгой проверки
4. Для production использовать OIDC discovery (автоматическое обновление ключей)
5. Логировать все аутентификации для аудита


## Настройка конфигурации

### Структура конфигурационного файла

Конфигурация Stronghold Agent описывается в HCL формате:

```hcl
# Подключение к Stronghold серверу
stronghold {
  address = "https://stronghold.example.com:8200"
  ca_cert = "/etc/stronghold-agent/ca.pem"
  
  # Настройки retry
  retry {
    num_retries = 5
  }
}

# Автоматическая аутентификация
auto_auth {
  method "approle" {
    # ... конфигурация метода
  }
  
  sink "file" {
    # ... конфигурация sink
  }
}

# API Proxy (опционально)
api_proxy {
  use_auto_auth_token = true
}

# Кеширование (опционально)
cache {
  use_auto_auth_token = true
}

# Listener для API Proxy
listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
}

# Шаблоны
template {
  source      = "/path/to/template.ctmpl"
  destination = "/path/to/output"
  # ... дополнительные настройки
}

# PID файл
pid_file = "/var/run/stronghold-agent.pid"

# Логирование
log_level = "info"
log_file = "/var/log/stronghold-agent.log"
```

### Секция vault/stronghold

Настройки подключения к Stronghold серверу используется имя секции `stronghold`. Если требуется интеграция с Hashicorp Vault, то следует использовать `vault` в качестве имени секции.

```hcl
stronghold {
  # URL сервера (обязательно)
  address = "https://stronghold.example.com:8200"
  
  # TLS настройки
  ca_cert = "/etc/stronghold-agent/ca.pem"              # CA сертификат
  ca_path = "/etc/stronghold-agent/ca-bundle/"          # Директория с CA
  client_cert = "/etc/stronghold-agent/client.pem"      # Клиентский сертификат
  client_key = "/etc/stronghold-agent/client-key.pem"   # Клиентский ключ
  tls_skip_verify = false                         # Не отключать в продакшене!
  tls_server_name = "stronghold.example.com"      # SNI имя
  
  # Retry политика
  retry {
    num_retries = 5  # Количество повторных попыток
  }
}
```

### Секция auto_auth

Настройка автоматической аутентификации:

```hcl
auto_auth {
  # Метод аутентификации
  method "approle" {
    mount_path = "auth/approle"  # Путь монтирования auth метода
    namespace  = "myns"           # Namespace (опционально, для Enterprise)
    
    config = {
      # Параметры специфичные для метода
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
    }
  }
  
  # Sink - куда сохранять токен (опционально, может быть несколько)
  sink "file" {
    config = {
      path = "/var/run/stronghold-agent/token"
      mode = 0640
    }
  }
  
  # Sink с шифрованием
  sink "file" {
    wrap_ttl = "5m"                    # Обернуть токен с TTL
    aad_env_var = "VAULT_AAD"          # Дополнительные данные для шифрования
    dh_type = "curve25519"             # Тип Diffie-Hellman
    dh_path = "/etc/stronghold-agent/dh-pub" # Публичный ключ
    
    config = {
      path = "/var/run/stronghold-agent/encrypted-token"
    }
  }
}
```

### Секция template

Настройка рендеринга шаблонов:

```hcl
template {
  # Путь к файлу шаблона (обязательно)
  source = "/etc/myapp/config.ctmpl"
  
  # Путь к выходному файлу (обязательно)
  destination = "/etc/myapp/config.conf"
  
  # Права доступа к файлу
  perms = "0600"
  
  # Пользователь и группа
  user = "myapp"
  group = "myapp"
  
  # Резервная копия перед заменой
  backup = true
  
  # Команда для выполнения после рендеринга
  command = "systemctl reload myapp"
  command_timeout = "30s"
  
  # Ждать перед первым рендерингом
  wait {
    min = "5s"
    max = "10s"
  }
  
  # Обработка ошибок
  error_on_missing_key = true
  
  # Рендерить только при изменениях
  create_dest_dirs = true
}
```

### Секция template_config

Глобальные настройки для всех шаблонов:

```hcl
template_config {
  # Выйти после первого рендеринга всех шаблонов
  exit_on_retry_failure = false
  
  # Интервал периодического рендеринга "статических" секретов (например KV).
  # Можно задавать строкой длительности (например "5m") или числом секунд.
  static_secret_render_interval = "5m"
}
```

### Секция exec (Process Supervisor)

Запуск дочернего процесса с инжектированными секретами:

```hcl
exec {
  # Команда для запуска (обязательно)
  command = ["/usr/bin/myapp", "--config", "/etc/myapp/config.yaml"]
  
  # Политика перезапуска при изменении секретов
  restart_on_secret_changes = "always"  # always, never
  
  # Сигнал для остановки процесса
  restart_stop_signal = "SIGTERM"
}

# Шаблон для переменных окружения
env_template "DATABASE_URL" {
  contents = "{{ with secret \"secret/data/myapp\" }}postgresql://{{ .Data.data.username }}:{{ .Data.data.password }}@db:5432{{ end }}"
  error_on_missing_key = true
}

env_template "API_KEY" {
  contents = "{{ with secret \"secret/data/myapp\" }}{{ .Data.data.api_key }}{{ end }}"
  error_on_missing_key = true
}
```

### Секция listener

Настройка HTTP(S) listener для API Proxy:

```hcl
listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
  
  # TLS настройки
  tls_cert_file = "/etc/stronghold-agent/agent-cert.pem"
  tls_key_file = "/etc/stronghold-agent/agent-key.pem"
  
  # Требовать специальный заголовок
  require_request_header = true
  
  # API настройки
  agent_api {
    enable_quit = true  # Включить /agent/v1/quit endpoint
  }
}

# Unix socket listener
listener "unix" {
  address = "/var/run/stronghold-agent.sock"
  tls_disable = true
  socket_mode = "0660"
  socket_user = "myapp"
  socket_group = "myapp"
}
```

### Логирование и отладка

```hcl
# Уровень логирования: trace, debug, info, warn, error
log_level = "info"

# Файл логов
log_file = "/var/log/stronghold-agent.log"

# Формат логов: standard, json
log_format = "json"

# Ротация логов
log_rotate_duration = "24h"
log_rotate_bytes = 104857600  # 100MB
log_rotate_max_files = 10
```

## Запуск и управление

### Проверка конфигурации

Перед запуском в продакшене **обязательно** проверьте корректность конфигурации:

**Метод 1: Пробный запуск с автоматическим завершением (рекомендуется)**

```bash
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -exit-after-auth -log-level=debug
```

Эта команда:
1. Проверяет синтаксис HCL конфигурации
2. Подключается к Stronghold серверу
3. Выполняет полную аутентификацию
4. Создает файлы/шаблоны
5. Автоматически завершается (не нужен Ctrl+C)

**Успешный результат:**

```log
[INFO]  agent: loaded config: path=/etc/stronghold-agent/agent.hcl
[INFO]  agent.auto_auth.approle: authentication successful
[INFO]  agent.sink.file: writing token to: /var/run/stronghold-agent/token
[INFO]  agent: exit after auth set, exiting
```

### Запуск в режиме разработки

Для отладки можно запускать Agent в foreground режиме:

```bash
# Базовый запуск
stronghold-agent -config=/etc/stronghold-agent/agent.hcl

# С повышенным уровнем логирования
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -log-level=debug

# Выход после первой успешной аутентификации (для проверки)
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -exit-after-auth
```

### Запуск Agent как systemd сервис

Создайте systemd unit файл `/etc/systemd/system/stronghold-agent.service`:

```ini
[Unit]
Description=Stronghold Agent
Documentation=https://docs.stronghold.example.com/agent
Requires=network-online.target
After=network-online.target
ConditionFileNotEmpty=/etc/stronghold-agent/agent.hcl

[Service]
Type=notify
User=stronghold-agent
Group=stronghold-agent
ExecStart=/usr/local/bin/stronghold-agent -config=/etc/stronghold-agent/agent.hcl
ExecReload=/bin/kill -HUP $MAINPID
KillMode=process
KillSignal=SIGTERM
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
# ВАЖНО: добавьте сюда все директории, куда Agent будет писать (template.destination, sink file, unix socket, логи).
# Пример ниже — базовый; расширяйте под вашу конфигурацию (например, /etc/myapp или /var/lib/myapp):
ReadWritePaths=/var/run/stronghold-agent /var/log/stronghold-agent /etc/myapp
CapabilityBoundingSet=CAP_IPC_LOCK

[Install]
WantedBy=multi-user.target
```

Управление сервисом:

```bash
# Перезагрузить systemd
sudo systemctl daemon-reload

# Запустить Agent
sudo systemctl start stronghold-agent

# Включить автозапуск
sudo systemctl enable stronghold-agent

# Проверить статус
sudo systemctl status stronghold-agent

# Просмотр логов
sudo journalctl -u stronghold-agent -f

# Перезагрузка конфигурации (SIGHUP)
sudo systemctl reload stronghold-agent

# Остановка
sudo systemctl stop stronghold-agent
```

{% endraw %}
