---
title: "Настройка интеграций"
permalink: ru/admin/configuration/monitoring/alerts-integrations.html
description: "Настройка интеграций алертов в Deckhouse Kubernetes Platform с Zabbix, Slack, Telegram и другими системами. Маршрутизация алертов, настройка уведомлений и интеграция с системами мониторинга."
lang: ru
---

{% raw %}

## Перенаправление алертов в Zabbix

Deckhouse Kubernetes Platform поддерживает интеграцию с системой мониторинга Zabbix. Для этого используется внешний скрипт, который получает алерты из Deckhouse через `kubectl` и передаёт их в Zabbix с помощью Zabbix-агента.

Для работы скрипта потребуются:

- установленный и настроенный zabbix-agent;
- утилиты `bash`, `jq`;
- `kubectl` с рабочим конфигурационным файлом и правами на выполнение команды `kubectl get clusteralerts`.

### Установка

1. Импортируйте шаблон в Zabbix:
   - В веб-интерфейсе Zabbix перейдите в раздел «Data collection → Templates»
   - Нажмите «Import» и загрузите файл `zbx_export_templates.yaml`:

     ```yaml
     zabbix_export:
       version: '6.4'
       template_groups:
         - uuid: 7df96b18c230490a9a0a9e2307226338
           name: Templates
       templates:
         - uuid: 91d9cf9d023749a3997bc6429ef45fe1
           template: 'Module d8alerts'
           name: 'Deckhouse Alerts'
           groups:
             - name: Templates
           discovery_rules:
             - uuid: e9be18e9305e48e887c43e1ad9ea9c31
               name: 'Deckhouse Alert Discovery'
               key: d8alerts.discovery
               lifetime: 3d
               item_prototypes:
                 - uuid: c4152d379d7a46cab23f7437cdb632c1
                   name: 'Deckhouse alert: {#ALERTNAME} severity'
                   key: 'd8alerts.severity[{#ALERTID}]'
                   history: 1d
                   trends: '0'
                   description: |
                     Summary: {#SUMMARY}
                
                     {#DESCRIPTION}
                
                     Labels: {#LABELS}
                     To get more information: `kubectl describe clusteralert {#ALERTID}`
                   preprocessing:
                     - type: REGEX
                       parameters:
                         - (\d+)
                         - \1
                   tags:
                     - tag: Application
                       value: deckhouse
                   trigger_prototypes:
                     - uuid: b27f8d03dc4c4d33bab4193a6ce57ae7
                       expression: 'last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) <= 2 and last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) > 0'
                       name: '{#ALERTNAME} is firing'
                       priority: DISASTER
                       description: |
                         Summary: {#SUMMARY}
                    
                         {#DESCRIPTION}
                    
                         Labels: {#LABELS}
                         To get more information: `kubectl describe clusteralert {#ALERTID}`
                     - uuid: d1935044cf3f49df8bc53f738e36b683
                       expression: 'last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) >= 3 and last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) <= 4'
                       name: '{#ALERTNAME} is firing'
                       priority: HIGH
                       description: |
                         Summary: {#SUMMARY}
                    
                         {#DESCRIPTION}
                    
                         Labels: {#LABELS}
                         To get more information: `kubectl describe clusteralert {#ALERTID}`
                     - uuid: 0b134c599d9d4de79f906fbd8e749ec2
                       expression: 'last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) >= 5 and last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) <= 6'
                       name: '{#ALERTNAME} is firing'
                       priority: AVERAGE
                       description: |
                         Summary: {#SUMMARY}
                    
                         {#DESCRIPTION}
                    
                         Labels: {#LABELS}
                         To get more information: `kubectl describe clusteralert {#ALERTID}`
                     - uuid: 6967dc80d6414239b2d597447815048a
                       expression: 'last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) >= 7 and last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) <= 8'
                       name: '{#ALERTNAME} is firing'
                       priority: WARNING
                       description: |
                         Summary: {#SUMMARY}
                    
                         {#DESCRIPTION}
                    
                         Labels: {#LABELS}
                         To get more information: `kubectl describe clusteralert {#ALERTID}`
                     - uuid: 8e531a2e6a0f47849c1dffea9a1733e1
                       expression: 'last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) >= 9 and last(/Module d8alerts/d8alerts.severity[{#ALERTID}]) <= 10'
                       name: '{#ALERTNAME} is firing'
                       priority: INFO
                       description: |
                         Summary: {#SUMMARY}
                    
                         {#DESCRIPTION}
                    
                         Labels: {#LABELS}
                         To get more information: `kubectl describe clusteralert {#ALERTID}`
      ```

   - После установки шаблона укажите ему группу, соответствующую агентам, с которых будет происходить сбор метрик.

1. Настройте Zabbix-агент:
   - Скопируйте файл `d8alerts.conf` в директорию, указанную в параметре `Include` основного конфига Zabbix-агента (обычно расположен по пути `/etc/zabbix/zabbix_agentd.d/`):

     ```console
     # LLD of deckhouse cluster alerts
     UserParameter=d8alerts.discovery,/etc/zabbix/scripts/clusteralerts.sh discovery

     # Severity of a specific alert by its ID
     UserParameter=d8alerts.severity[*],/etc/zabbix/scripts/clusteralerts.sh severity "$1"
     ```

   - Скопируйте скрипт `clusteralerts.sh` в директорию `/etc/zabbix/scripts/`:

     ```console
     #!/bin/bash

     MODE="$1"
     ALERT_ID="$2"

     get_alerts_json() {
       kubectl get clusteralerts -o json
     }

     get_alert_by_id() {
       local id="$1"
       kubectl get clusteralerts "$id" -o json
     }

     if [[ "$MODE" == "discovery" ]]; then
       get_alerts_json | jq -c '{
           data: [.items[] | {
             "{#ALERTID}": .metadata.name,
             "{#ALERTNAME}": .alert.name,
             "{#SEVERITY}": (.alert.severityLevel | tonumber),
             "{#DESCRIPTION}": .alert.description,
             "{#SUMMARY}": .alert.summary,
             "{#LABELS}": (.alert.labels | to_entries | map("\(.key)=\(.value)") | join(","))
           }]
         }'

     elif [[ "$MODE" == "severity" && -n "$ALERT_ID" ]]; then
       get_alert_by_id "$ALERT_ID" | jq -r '.alert.severityLevel'

     else
       echo "Invalid usage"
       exit 1
     fi
     ```

   - Убедитесь, что скрипт имеет права на выполнение:

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

   Убедитесь, что скрипт корректно выполняется и возвращает ожидаемые данные.

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

Разверните [кастомный ресурс CustomAlertManager](/modules/prometheus/cr.html#customalertmanager):

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

## Пример отправки алертов по электронной почте

Создайте секрет с паролем от аккаунта электронной почты. Пароль, закодированный в формате Base64, укажите в поле `password`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: am-mail-server-pass
  namespace: d8-monitoring
data:
  password: BASE64_ENCODED_PASSWORD_HERE
```

Измените значения в примере [ресурса CustomAlertManager](/modules/prometheus/cr.html#customalertmanager) в соответствии с актуальными для вашей инфраструктуры значениями и примените его:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: mail
spec:
  type: Internal
  internal:
    receivers:
      - name: devnull
      - name: mail
        emailConfigs:
          - to: oncall@example.com
            from: prom@example.com
            smarthost: mx.example.com:587
            authIdentity: prom@example.com
            authUsername: prom@example.com
            authPassword:
              key: password
              name: am-mail-server-pass
            # Если вы используете custom CA на сервере, можете поместить публичную часть CA в ConfigMap в пространстве имен d8-monitoring
            # tlsConfig:
            #   insecureSkipVerify: true
            #   ca:
            #     configMap:
            #       key: ca.pem
            #       name: alertmanager-mail-server-ca
            sendResolved: true
            requireTLS: true
    route:
      groupBy:
        - job
      groupInterval: 5m
      groupWait: 30s
      receiver: devnull
      repeatInterval: 24h
      routes:
        - matchers:
          - matchType: =~
            name: severity_level
            value: "^[1-4]$"
          receiver: mail
```

{% endraw %}
