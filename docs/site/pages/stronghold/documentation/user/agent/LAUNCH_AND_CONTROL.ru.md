---
title: "Запуск и управление"
permalink: ru/stronghold/documentation/agent/launch-and-control.html
lang: ru
---

## Запуск и управление

### Проверка конфигурации

Перед запуском в production **обязательно** проверьте корректность конфигурации.
Рекомендуется пробный запуск с автоматическим завершением:

```bash
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -exit-after-auth -log-level=debug
```

Эта команда:
1. Проверяет синтаксис HCL конфигурации.
2. Подключается к Stronghold серверу.
3. Выполняет полную аутентификацию.
4. Создает файлы/шаблоны.
5. Автоматически завершается (не нужен Ctrl+C).

**Успешный результат:**

```text
[INFO]  agent: loaded config: path=/etc/stronghold-agent/agent.hcl
[INFO]  agent.auto_auth.approle: authentication successful
[INFO]  agent.sink.file: writing token to: /var/run/stronghold-agent/token
[INFO]  agent: exit after auth set, exiting
```

### Запуск в режиме разработки

Для отладки можно запускать Agent в foreground режиме:

```bash
# Базовый запуск.
stronghold-agent -config=/etc/stronghold-agent/agent.hcl

# С повышенным уровнем логирования.
stronghold-agent -config=/etc/stronghold-agent/agent.hcl -log-level=debug

# Выход после первой успешной аутентификации (для проверки).
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
# Перезагрузите systemd.
sudo systemctl daemon-reload

# Запустите Agent.
sudo systemctl start stronghold-agent

# Включите автозапуск.
sudo systemctl enable stronghold-agent

# Проверьте статус.
sudo systemctl status stronghold-agent

# Просмотр логов.
sudo journalctl -u stronghold-agent -f

# Перезагрузка конфигурации (SIGHUP).
sudo systemctl reload stronghold-agent

# Остановка.
sudo systemctl stop stronghold-agent
```

{% endraw %}
