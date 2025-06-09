---
title: Настройка алертов
permalink: ru/admin/configuration/monitoring/alerts.html
lang: ru
---

{% raw %}

## Перенаправление алертов в Zabbix

Deckhouse поддерживает интеграцию с системой мониторинга Zabbix. Для этого используется внешний скрипт, который получает алерты из Deckhouse через `kubectl` и передаёт их в Zabbix с помощью Zabbix-агента.

Для работы скрипта потребуются:

- установленный и настроенный zabbix-agent;
- утилиты `bash`, `jq`;
- `kubectl` с рабочим конфигурационным файлом и правами на выполнение команды `kubectl get clusteralerts`.

### Установка

1. Импортируйте шаблон в Zabbix:
   - В веб-интерфейсе Zabbix перейдите в раздел «Data collection → Templates»
   - Нажмите «Import» и загрузите файл `zbx_export_templates.yaml`
   - После установки шаблона укажите ему группу, соответствующую агентам, с которых будет происходить сбор метрик.

1. Настройте Zabbix-агент:
   - Скопируйте файл `d8alerts.conf` в директорию, указанную в параметре `Include` основного конфига Zabbix-агента (обычно расположен по пути `/etc/zabbix/zabbix_agentd.d/`)
   - Скопируйте скрипт `clusteralerts.sh` в директорию `/etc/zabbix/scripts/` и убедитесь, что он имеет права на выполнение:

     ```console
     chmod +x /etc/zabbix/scripts/clusteralerts.sh
     ```

1. Проверьте доступ:
   - Убедитесь, что скрипт имеет доступ к кластеру и может получать информацию об алертах:

     ```console
     /etc/zabbix/scripts/clusteralerts.sh discovery
     ```

     Вывод должен содержать список алертов с их статусами и уровнями критичности.

1. Перезапустите Zabbix-агент для применения изменений:

   ```console
   systemctl restart zabbix-agent
   ```

### Устранение неполадок

1. Проверьте логи Zabbix-агента:

   ```console
   tail -f /var/log/zabbix/zabbix_agentd.log
   ```

1. Проверьте работу скрипта:

   ```console
   /etc/zabbix/scripts/clusteralerts.sh discovery
   /etc/zabbix/scripts/clusteralerts.sh severity "ID_АЛЕРТА"
   ```

   Убедитесь, что скрипт корректно выполняется и возвращает ожидаемые данные:

1. Проверьте права пользователя `zabbix`.  Убедитесь, что агент выполняет скрипт от пользователя с нужными правами доступа к кластеру:

   - Скрипт запускается от имени пользователя `zabbix`:

     ```console
     sudo -u zabbix /etc/zabbix/scripts/clusteralerts.sh
     ```

   - Переменная `KUBECONFIG` доступна пользователю:

     Если конфигурационный файл Kubernetes недоступен по умолчанию, укажите его явно. Для этого сохраните kubeconfig, например, в `/etc/zabbix/kubeconfig` и добавьте в конфигурацию агента:

     ```console
     UserParameter=d8alerts.discovery,export KUBECONFIG=/etc/zabbix/kubeconfig;/etc/zabbix/scripts/clusteralerts.sh discovery
     ```

## Отправка алертов в Telegram

Alertmanager поддерживает прямую отправку алертов в Telegram.

Создайте Secret в пространстве имен `d8-monitoring`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: telegram-bot-secret
  namespace: d8-monitoring
stringData:
  token: "562696849:AAExcuJ8H6z4pTlPuocbrXXXXXXXXXXXx"
```

Разверните кастомный ресурс `CustomAlertManager`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: telegram
spec:
  type: Internal
  internal:
    receivers:
      - name: telegram
        telegramConfigs:
          - botToken:
              name: telegram-bot-secret
              key: token
            chatID: -30490XXXXX
    route:
      groupBy:
        - job
      groupInterval: 5m
      groupWait: 30s
      receiver: telegram
      repeatInterval: 12h
```

Поля `token` в Secret'е и `chatID` в ресурсе `CustomAlertmanager` необходимо поставить свои. [Подробнее](https://core.telegram.org/bots) о Telegram API.

## Пример отправки алертов в Slack с фильтром

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: slack
spec:
  internal:
    receivers:
    - name: devnull
    - name: slack
      slackConfigs:
      - apiURL:
          key: apiURL
          name: slack-apiurl
        channel: {{ dig .Values.werf.env .Values.slack.channel._default .Values.slack.channel }} 
        fields:
        - short: true
          title: Severity
          value: '{{`{{  .CommonLabels.severity_level }}`}}'
        - short: true
          title: Status
          value: '{{`{{ .Status }}`}}'
        - title: Summary
          value: '{{`{{ range .Alerts }}`}}{{`{{ .Annotations.summary }}`}} {{`{{ end }}`}}'
        - title: Description
          value: '{{`{{ range .Alerts }}`}}{{`{{ .Annotations.description }}`}} {{`{{ end }}`}}'
        - title: Labels
          value: '{{`{{ range .Alerts }}`}} {{`{{ range .Labels.SortedPairs }}`}}{{`{{ printf "%s:
            %s\n" .Name .Value }}`}}{{`{{ end }}`}}{{`{{ end }}`}}'
        - title: Links
          value: '{{`{{ (index .Alerts 0).GeneratorURL }}`}}'
        title: '{{`{{ .CommonLabels.alertname }}`}}'
    route:
      groupBy:
      - '...'  
      receiver: devnull
      routes:
        - matchers:
          - matchType: =~
            name: severity_level
            value: "^[4-9]$"
          receiver: slack
      repeatInterval: 12h
  type: Internal
```

## Пример отправки алертов в Opsgenie

```yaml
- name: opsgenie
        opsgenieConfigs:
          - apiKey:
              key: data
              name: opsgenie
            description: |
              {{ range .Alerts }}{{ .Annotations.summary }} {{ end }}
              {{ range .Alerts }}{{ .Annotations.description }} {{ end }}
            message: '{{ .CommonLabels.alertname }}'
            priority: P1
            responders:
              - id: team_id
                type: team
```

{% endraw %}
