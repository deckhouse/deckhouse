---
title: "Модуль admission-policy-engine"
---

Позволяет использовать в кластере политики безопасности согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)  Kubernetes. Модуль для работы использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

Pod Security Standards определяют три политики, которые охватывают весь спектр безопасности. Эти политики являются кумулятивными политиками, т.е. состоящими из набора политик, и варьируются по уровню ограничений от неограничивающего до ограничивающего значительно.

Список политик, предлагаемых модулем для использования:
- `Privileged` — неограничивающая политика, предоставляющая максимально широкий уровень разрешений (используется по умолчанию).
- `Baseline` — минимально ограничивающая политика, которая предотвращает использование наиболее известных способов повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию Pod'а.
- `Restricted` — политика со значительными ограничениями, обеспечивает самые жесткие требования к Pod'ам.

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

Для применения политики достаточно установить label `security.deckhouse.io/pod-policy=<POLICY_NAME>` на Namespace.

Пример установки политики `Restricted` для всех Pod'ов в Namespace `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```
