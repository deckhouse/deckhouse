---
title: "Модуль neuvector: примеры использования"
---

## Пример политики безопасности (Security Rules)

```yaml
target:
  policymode: Protect  # варианты: Discover, Monitor, Protect
  selector:
    name: group-name
    criteria:
      - key: service
        value: service-name
        op: "="
      - key: domain
        value: namespace
        op: "="
```

здесь:
- `Protect`: блокирует нарушения.
- `Monitor`: только логирует.
- `Discover`: обучающий режим (ничего не блокирует).

## Пример сетевого правила (Network Rules)

```yaml
ingress:
  - action: allow
    name: allow-web-traffic
    selector:
      name: frontend-group
      criteria:
        - key: app
          value: frontend
          op: "="
    ports: "tcp/80,tcp/443"
    applications: ["HTTP", "SSL"]
    priority: 0
```

## Пример правила для процессов (Process Rules)

```yaml
process:
  - action: allow
    name: nginx-process
    path: /usr/sbin/nginx
    allow_update: false
```

## Пример правила доступа к файлам (File Access Rules)

```yaml
file:
  - behavior: block_access
    filter: /etc/shadow
    recursive: false
    app: ["nginx"]
```

## Профиль процессов (Process Profiles)

```yaml
process_profile:
  baseline: zero-drift
  mode: Protect
```

## Организация политик

1. Используйте NvGroupDefinition для переиспользуемых групп:

   ```yaml
   # Определить один раз
   apiVersion: neuvector.com/v1
   kind: NvGroupDefinition
   metadata:
     name: web-tier
   spec:
     selector:
       name: nv.web-tier.production
   ```

1. Используйте правила с областью действия пространства имен для политик приложений:

   ```yaml
   # Специфично для приложения в пространстве имен
   apiVersion: neuvector.com/v1
   kind: NvSecurityRule
   metadata:
     name: app-security
     namespace: production
   ```

1. Используйте правила на уровне кластера для глобальных политик:

   ```yaml
   # Глобальная базовая безопасность
   apiVersion: neuvector.com/v1
   kind: NvClusterSecurityRule
   metadata:
     name: baseline-security
   ```

## Миграция политики из staging в production

```bash
# Экспорт из staging.
kubectl get nvsecurityrule -o yaml > staging-policies.yaml

# Редактирование режима.
target:
  policymode: Protect  # Было: Monitor.

# Применение в production.
kubectl apply -f production-policies.yaml
```

## Пример интеграции с RBAC (RBAC Integration)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: neuvector-policy-manager
rules:
- apiGroups: ["neuvector.com"]
  resources: ["nvsecurityrules", "nvclustersecurityrules"]
  verbs: ["get", "list", "create", "update", "delete"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dev-team-policies
  namespace: development
subjects:
- kind: User
  name: dev-team-lead
roleRef:
  kind: ClusterRole
  name: neuvector-policy-manager
```

## Интеграция с GitOps (GitOps Workflow)

```yaml
# .github/workflows/security-policies.yml
name: Deploy Security Policies
on:
  push:
    paths: ['policies/**']
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Deploy policies
        run: kubectl apply -f policies/
```

## Интеграция в CI/CD (скрипт в Bash)

```bash
# !/bin/bash
# validate-policies.sh
set -e

echo "Validating NeuVector policies..."
for policy in policies/*.yaml; do
  kubectl apply --dry-run=client -f "$policy"
done

echo "Applying policies to staging..."
kubectl apply -f policies/ --namespace=staging

echo "Running security tests..."
./run-security-tests.sh

echo "Promoting to production..."
kubectl apply -f policies/ --namespace=production
```
