---
title: "Сценарии использования"
permalink: ru/stronghold/documentation/agent/use-cases.html
lang: ru
---

## Развертывание на VM и Bare-Metal

**Основной сценарий использования Stronghold Agent** — это развертывание на виртуальных машинах (VM) и физических серверах (bare-metal).
В отличие от Kubernetes, где есть нативные механизмы работы с секретами через CSI-драйверы и sidecar-контейнеры, на VM/bare-metal требуется отдельное решение.

**Преимущества:**
- Централизованное управление секретами для всего парка серверов.
- Простая интеграция через systemd сервис.
- Автоматическое обновление секретов без простоя.
- Поддержка legacy систем без необходимости изменения кода приложений.

## Доставка секретов в Legacy приложения

Legacy приложения часто ожидают конфигурацию в виде:
- **Файлов конфигурации** (config.ini, application.properties, .env).
- **Переменных окружения** (ENV vars).
- **Файлов ключей и сертификатов**.

Stronghold Agent может:
- Рендерить шаблоны с секретами в файлы.
- Запускать приложения с инжектированными переменными окружения.
- Автоматически перезапускать приложение при обновлении секретов.

**Пример:** Spring Boot приложение получает credentials через переменные окружения.

Agent автоматически создает переменные окружения с секретами:

```bash
DB_USERNAME=v-approle-myapp-abc123
DB_PASSWORD=A1b2C3d4E5f6
DB_HOST=postgres.example.com
```

В `application.properties` просто используйте эти переменные:

```java
spring.datasource.url=jdbc:postgresql://${DB_HOST}:5432/production
spring.datasource.username=${DB_USERNAME}
spring.datasource.password=${DB_PASSWORD}
```

Приложение не требует изменений - Spring Boot автоматически подставит значения из ENV.

## Интеграция с приложениями без SDK поддержки

Многие приложения написаны на языках или фреймворках, для которых нет готовых SDK для работы со Stronghold:
- Legacy приложения C/C++.
- Специализированные системы (SCADA, промышленное ПО).
- Бинарные приложения без исходного кода.

Stronghold Agent позволяет таким приложениям использовать секреты через стандартные механизмы ОС.

## Автоматическое обновление credentials без перезапуска

Stronghold Agent может отслеживать изменения секретов и:
- **Обновлять файлы** с новыми значениями.
- **Отправлять сигналы** приложению (SIGHUP для перезагрузки конфигурации).
- **Выполнять команды** (скрипты перезагрузки, hot-reload).
- **Перезапускать процессы** при критических изменениях.

**Пример:** Веб-сервер Nginx получает обновленные TLS сертификаты:

```hcl
template {
  source      = "/etc/nginx/ssl/cert.ctmpl"
  destination = "/etc/nginx/ssl/cert.pem"
  command     = "nginx -s reload"  # Перезагрузка без простоя
}
```

## Получение динамических секретов

Stronghold поддерживает динамическую генерацию временных credentials для различных систем:

**Database credentials:**
- PostgreSQL, MySQL, MongoDB, Oracle.
- Временные пользователи с ограниченным TTL.
- Автоматическая ротация.

**PKI сертификаты:**
- Автоматическая выдача TLS сертификатов.
- Обновление перед истечением срока действия.
- Поддержка различных CA.

## Использование в CI/CD пайплайнах

Для self-hosted CI/CD runners (Jenkins, GitLab Runner) Agent запускается на сервере runner'а и предоставляет секреты для пайплайнов.

**Как это работает:**
1. Agent устанавливается на сервер CI/CD runner.
1. Аутентифицируется в Stronghold через AppRole.
1. Рендерит секреты в файлы или предоставляет через API Proxy.
1. Пайплайн читает секреты из локальных файлов.

**Типичные сценарии:**
- **Деплой credentials** - SSH ключи, kubeconfig для развертывания.
- **Registry access** - Docker registry credentials для pull/push образов.
- **Cloud providers** - Облачные credentials для инфраструктуры.

**Пример для GitLab Runner:**

```yaml
# .gitlab-ci.yml
deploy:
  script:
    - ssh -i /var/run/stronghold-agent/deploy_key deploy@server.example.com "cd /app && git pull && systemctl restart app"
```

**Роль Stronghold Agent:**

- Agent получает SSH ключ из Stronghold (например, из secret/data/ci/deploy_key).
- Рендерит его в файл /var/run/stronghold-agent/deploy_key с правами 0600.
- При ротации ключа в Stronghold - agent автоматически обновляет файл.
- GitLab Runner просто использует этот ключ - он всегда актуальный.
