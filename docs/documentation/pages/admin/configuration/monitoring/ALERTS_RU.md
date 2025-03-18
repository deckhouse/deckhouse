---
title: Настройка алертов
permalink: ru/admin/configuration/monitoring/alerts.html
lang: ru
---

{% raw %}

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
