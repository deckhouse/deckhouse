# CSE edit

## monitoring/grafana-dashboards/security

Dashboards for viewing security events have been added.

## monitoring/prometheus-rules/security-events/security-events-sync.yaml

Adds an alert for security events.

## openapi/config-values.yaml

remove allowDeleteLogs

We do not allow logs to be deleted. We chose the OpenAPI option because it cannot be bypassed without rebuilding the controller image.

ValidatingAdmissionPolicy is not suitable because it can be bypassed if someone is determined to do so.

## templates/security-events.yaml

Adds rules for collecting and transforming logs into security events

## templates/audit-logs.yaml

Adds log collection from /var/log/kube-audit/audit.log
