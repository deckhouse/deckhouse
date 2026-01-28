---
title: "Configuring integrations"
permalink: en/admin/configuration/monitoring/alerts-integrations.html
description: "Configure alert integrations in Deckhouse Kubernetes Platform with Zabbix, Slack, Telegram, and other systems. Alert routing, notification setup, and monitoring system integration."
---

{% raw %}

## Redirecting alerts to Zabbix

Deckhouse Kubernetes Platform supports integration with the Zabbix monitoring system. For this purpose, an external script is used that receives alerts from Deckhouse via `kubectl` and sends them to Zabbix using the Zabbix agent.

The script requires:

- installed and configured zabbix-agent;
- utilities `bash`, `jq`;
- `kubectl` with a working configuration file and permissions to execute the `kubectl get clusteralerts` command.

### Installation

1. Import the template into Zabbix:
   - In the Zabbix web interface, go to the "Data collection → Templates" section
   - Click "Import" and upload the `zbx_export_templates.yaml` file:

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

   - After installing the template, assign it to the group corresponding to the agents from which metrics will be collected.

1. Configure the Zabbix agent:
   - Copy the `d8alerts.conf` file to the directory specified in the `Include` parameter of the main Zabbix agent config (usually located at `/etc/zabbix/zabbix_agentd.d/`):

     ```console
     # LLD of deckhouse cluster alerts
     UserParameter=d8alerts.discovery,/etc/zabbix/scripts/clusteralerts.sh discovery

     # Severity of a specific alert by its ID
     UserParameter=d8alerts.severity[*],/etc/zabbix/scripts/clusteralerts.sh severity "$1"
     ```

   - Copy the `clusteralerts.sh` script to the `/etc/zabbix/scripts/` directory:

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

   - Make sure the script has execute permissions:

     ```console
     chmod +x /etc/zabbix/scripts/clusteralerts.sh
     ```

1. Check access:
   - Ensure the script has access to the cluster and can retrieve alert information:

     ```console
     /etc/zabbix/scripts/clusteralerts.sh discovery
     ```

     The output should contain a list of alerts with their statuses and severity levels.

1. Restart the Zabbix agent to apply changes:

   ```console
   systemctl restart zabbix-agent
   ```

### Troubleshooting

1. Check Zabbix agent logs:

   ```console
   tail -f /var/log/zabbix/zabbix_agentd.log
   ```

1. Check script operation:

   ```console
   /etc/zabbix/scripts/clusteralerts.sh discovery
   /etc/zabbix/scripts/clusteralerts.sh severity "ALERT_ID"
   ```

   Make sure the script executes correctly and returns expected data.

1. Check the `zabbix` user permissions. Ensure the agent runs the script as a user with the necessary cluster access rights:

   - The script runs as the `zabbix` user:

     ```console
     sudo -u zabbix /etc/zabbix/scripts/clusteralerts.sh
     ```

   - The `KUBECONFIG` variable is available to the user:

     If the Kubernetes configuration file is not available by default, specify it explicitly. To do this, save the kubeconfig, for example, in `/etc/zabbix/kubeconfig` and add to the agent configuration:

     ```console
     UserParameter=d8alerts.discovery,export KUBECONFIG=/etc/zabbix/kubeconfig;/etc/zabbix/scripts/clusteralerts.sh discovery
     ```

## Sending alerts to Telegram

Alertmanager supports direct sending of alerts to Telegram.

Create a Secret in the `d8-monitoring` namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: telegram-bot-secret
  namespace: d8-monitoring
stringData:
  token: "562696849:AAExcuJ8H6z4pTlPuocbrXXXXXXXXXXXx"
```

Deploy the [CustomAlertManager](/modules/prometheus/cr.html#customalertmanager) custom resource:

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

The `token` field in the Secret and `chatID` in the `CustomAlertmanager` resource need to be set to your own values. [More details](https://core.telegram.org/bots) about the Telegram API.

## Example of sending alerts to Slack with filter

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

## Example of sending alerts to Opsgenie

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

## Example of sending alerts via email

Create a secret with the email account password. Specify the password encoded in Base64 format in the `password` field:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: am-mail-server-pass
  namespace: d8-monitoring
data:
  password: BASE64_ENCODED_PASSWORD_HERE
```

Modify the values in the [CustomAlertManager](/modules/prometheus/cr.html#customalertmanager) resource example according to the values relevant to your infrastructure and apply it:

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
            # If you use a custom CA on the server, you can place the public part of the CA in a ConfigMap in the d8-monitoring namespace
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
