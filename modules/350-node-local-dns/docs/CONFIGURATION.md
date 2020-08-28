---
title: "Модуль node-local-dns: конфигурация"
---

Модуль **включен** по умолчанию и не требует конфигурации – всё работает из коробки.

**Внимание!**
- Работает только для iptables режима `kube-proxy` (ipvs не поддерживается и поведение с ipvs не проверялось).
- По умолчанию **не работает** для запросов из `hostNetwork`, все запросы уходят в `kube-dns`. В данном случае можно самостоятельно в конфигурации пода указать адрес `169.254.20.10`, но тогда в случае падения `node-local-dns` не будет работать fallback на `kube-dns`.

## Примеры
### Пример настройки кастомного DNS в поде

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dns-example
spec:
  dnsPolicy: "None"
  dnsConfig:
    nameservers:
      - 169.254.20.10
  containers:
    - name: test
      image: nginx
```

[Подробнее](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config) про настройку DNS.
