---
title: "Настройка конфигурации"
permalink: ru/stronghold/documentation/agent/settings.html
lang: ru
---

{% raw %}

## Структура конфигурационного файла

Конфигурация Stronghold Agent описывается в HCL формате:

```hcl
# Подключение к Stronghold серверу.
stronghold {
  address = "https://stronghold.example.com:8200"
  ca_cert = "/etc/stronghold-agent/ca.pem"
  
  # Настройки retry.
  retry {
    num_retries = 5
  }
}

# Автоматическая аутентификация.
auto_auth {
  method "approle" {
    # ... конфигурация метода.
  }
  
  sink "file" {
    # ... конфигурация sink.
  }
}

# API Proxy (опционально).
api_proxy {
  use_auto_auth_token = true
}

# Кеширование (опционально).
cache {
  use_auto_auth_token = true
}

# Listener для API Proxy.
listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = true
}

# Шаблоны.
template {
  source      = "/path/to/template.ctmpl"
  destination = "/path/to/output"
  # ... дополнительные настройки.
}

# PID файл.
pid_file = "/var/run/stronghold-agent.pid"

# Логирование.
log_level = "info"
log_file = "/var/log/stronghold-agent.log"
```

## Секция vault/stronghold

Для подключения к Stronghold серверу используется имя секции `stronghold`. Если требуется интеграция с Hashicorp Vault, то следует использовать `vault` в качестве имени секции.

```hcl
stronghold {
  # URL сервера (обязательно).
  address = "https://stronghold.example.com:8200"
  
  # TLS настройки.
  ca_cert = "/etc/stronghold-agent/ca.pem"              # CA сертификат.
  ca_path = "/etc/stronghold-agent/ca-bundle/"          # Директория с CA.
  client_cert = "/etc/stronghold-agent/client.pem"      # Клиентский сертификат.
  client_key = "/etc/stronghold-agent/client-key.pem"   # Клиентский ключ.
  tls_skip_verify = false                         # Не отключать в продакшене!
  tls_server_name = "stronghold.example.com"      # SNI имя.
  
  # Retry политика,
  retry {
    num_retries = 5  # Количество повторных попыток.
  }
}
```

## Секция auto_auth

Настройка автоматической аутентификации:

```hcl
auto_auth {
  # Метод аутентификации.
  method "approle" {
    mount_path = "auth/approle"  # Путь монтирования auth метода
    namespace  = "myns"           # Namespace (опционально, для Enterprise)
    
    config = {
      # Параметры специфичные для метода.
      role_id_file_path = "/etc/stronghold-agent/role-id"
      secret_id_file_path = "/etc/stronghold-agent/secret-id"
    }
  }
  
  # Sink - куда сохранять токен (опционально, может быть несколько).
  sink "file" {
    config = {
      path = "/var/run/stronghold-agent/token"
      mode = 0640
    }
  }
  
  # Sink с шифрованием.
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

## Секция template

Настройка рендеринга шаблонов:

```hcl
template {
  # Путь к файлу шаблона (обязательно).
  source = "/etc/myapp/config.ctmpl"
  
  # Путь к выходному файлу (обязательно).
  destination = "/etc/myapp/config.conf"
  
  # Права доступа к файлу.
  perms = "0600"
  
  # Пользователь и группа.
  user = "myapp"
  group = "myapp"
  
  # Резервная копия перед заменой.
  backup = true
  
  # Команда для выполнения после рендеринга.
  command = "systemctl reload myapp"
  command_timeout = "30s"
  
  # Ожидание перед первым рендерингом.
  wait {
    min = "5s"
    max = "10s"
  }
  
  # Обработка ошибок.
  error_on_missing_key = true
  
  # Рендер только при изменениях.
  create_dest_dirs = true
}
```

## Секция template_config

Глобальные настройки для всех шаблонов:

```hcl
template_config {
  # Выйти после первого рендеринга всех шаблонов.
  exit_on_retry_failure = false
  
  # Интервал периодического рендеринга «статических» секретов (например KV).
  # Можно задавать строкой длительности (например "5m") или числом секунд.
  static_secret_render_interval = "5m"
}
```

### Секция exec (Process Supervisor)

Запуск дочернего процесса с инжектированными секретами:

```hcl
exec {
  # Команда для запуска (обязательно)Я.
  command = ["/usr/bin/myapp", "--config", "/etc/myapp/config.yaml"]
  
  # Политика перезапуска при изменении секретов.
  restart_on_secret_changes = "always"  # always, never
  
  # Сигнал для остановки процесса.
  restart_stop_signal = "SIGTERM"
}

# Шаблон для переменных окружения.
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
  
  # TLS настройки.
  tls_cert_file = "/etc/stronghold-agent/agent-cert.pem"
  tls_key_file = "/etc/stronghold-agent/agent-key.pem"
  
  # Требование специального заголовка.
  require_request_header = true
  
  # Настройки API.
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

## Логирование и отладка

```hcl
# Уровень логирования: trace, debug, info, warn, error.
log_level = "info"

# Файл логов.
log_file = "/var/log/stronghold-agent.log"

# Формат логов: standard, json.
log_format = "json"

# Ротация логов.
log_rotate_duration = "24h"
log_rotate_bytes = 104857600  # 100MB
log_rotate_max_files = 10
```

{% endraw %}
