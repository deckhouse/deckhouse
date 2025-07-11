---
title: "Модуль neuvector: примеры использования"
---

## Включение NeuVector

1. Включите модуль:

    ```bash
    d8 platform module enable neuvector
    ```

1. Настройте конфигурацию (используйте свои данные в полях `name`, `bootstrapPassword`, `host`):

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: neuvector
    spec:
      enabled: true
      settings:
        controller:
          ingress:
            enabled: true
            host: neuvector.example.com
        manager:
          ingress:
            enabled: true
            host: neuvector-ui.example.com
    ```

1. Получите доступ к интерфейсу управления:
  - Перейдите к настроенному имени хоста ingress.
  - Войдите с именем пользователя `admin` и вашим настроенным паролем.
  - Начните настройку политик безопасности и мониторинга.

## Настройка сканирования уязвимостей

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: neuvector
spec:
  settings:
    scanner:
      enabled: true
      replicas: 2
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
```

## Получение пароля

Если нужно получить пароль администратора, хранящийся в Kubernetes-секрете, в пространстве имен d8-neuvector, используйте следующую команду:

```txt
kubectl -n d8-neuvector get secret admin -o jsonpath='{.data.password}' | base64 -d
```
