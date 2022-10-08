---
title: "Модуль admission-policy-engine"
---

Обеспечивает политики безопасности в кластере с помощью [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/)

## Pod Security Standards

Pod Security Standards определяют три различных политики, которые широко охватывают спектр безопасности. Эти политики являются кумулятивными и варьируются от высокоразрешительных до высокоограничительных.

- Privileged - неограниченная политика, предоставляющая максимально широкий уровень разрешений (используется по-умолчанию).
- Baseline - Минимально ограничительная политика, которая предотвращает известное повышение привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию Pod.
- Restricted - политика со значительными ограничениями, обеспечивает самые жесткие требования к Pod'ам.

Подробнее про каждый набор политик можно прочитать [здесь](https://kubernetes.io/docs/concepts/security/pod-security-standards/)

Для применения данных политик нужно навесить label на желаемый namespace:
- `security.deckhouse.io/pod-policy=baseline`
- `security.deckhouse.io/pod-policy=restricted`

Например:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Установит политику `Restricted` для всех Pod в Namespace `my-namespace`
