---
title: "Модуль vertical-pod-autoscaler"
search: autoscaler
---

Vertical Pod Autoscaler ([VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)) — это инфраструктурный сервис, который позволяет не выставлять точные resource requests, если неизвестно, сколько ресурсов необходимо контейнеру для работы. При использовании VPA и включении соответствующего режима работы resource requests выставляются автоматически на основе потребления ресурсов (полученных данных из Prometheus).
Как вариант, возможно только получать рекомендации по ресурсам, без их автоматического изменения.

У VPA есть следующие режимы работы:
- `"Auto"` (default) — в данный момент режимы работы Auto и Recreate делают одно и то же. Однако, когда в kubernetes появится [Pod in-place resource update](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates), данный режим будет делать именно его.
- `"Recreate"` — данный режим разрешает VPA изменять ресурсы у запущенных подов, то есть рестартить их при работе. В случае работы одного пода (`replicas: 1`) это приведет к недоступности сервиса на время рестарта. В данном режиме VPA не пересоздает поды, которые были созданы без контроллера.
- `"Initial"` — VPA изменяет ресурсы подов только при создании подов, но не во время работы.
- `"Off"` — VPA не изменяет автоматически никакие ресурсы. В данном случае, если есть VPA c таким режимом работы, мы можем посмотреть, какие ресурсы рекомендует поставить VPA (kubectl describe vpa <vpa-name>).

Ограничения VPA:
- Обновление ресурсов запущенных подов — это экспериментальная возможность VPA. Каждый раз, когда VPA обновляет `resource requests` пода, под пересоздается. Соответственно, под может быть создан на другом узле.
- VPA **не должен использоваться с [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) по CPU и памяти** в данный момент. Однако VPA можно использовать с HPA на custom/external metrics.
- VPA реагирует почти на все `out-of-memory` events, но не гарантирует реакцию (почему так — выяснить из документации не удалось).
- Производительность VPA не тестировалась на огромных кластерах.
- Рекомендации VPA могут превышать доступные ресурсы в кластере, что **может приводить к подам в состоянии Pending**.
- Использование нескольких VPA-ресурсов над одним подом может привести к неопределенному поведению.
- В случае удаления VPA или его «выключения» (режим `Off`) изменения, внесенные ранее VPA, не сбрасываются, а остаются в последнем измененном значении. Из-за этого может возникнуть путаница, что в Helm будут описаны одни ресурсы, при этом в контроллере тоже будут описаны одни ресурсы, но реально у подов ресурсы будут совсем другие и может сложиться впечатление, что они взялись «непонятно откуда».

> **Важно!** При использовании VPA настоятельно рекомендуется использовать [Pod Disruption Budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/).

## Grafana dashboard

На досках:
- `Main / Namespace`, `Main / Namespace / Controller`, `Main / Namespace / Controller / Pod` — столбец `VPA type` показывает значение `updatePolicy.updateMode`;
- `Main / Namespaces` — столбец `VPA %` показывает процент подов с включенным VPA.

## Архитектура Vertical Pod Autoscaler

VPA состоит из 3 компонентов:
- `Recommender` — мониторит настоящее (делая запросы в [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md), который реализован в модуле [`prometheus-metrics-adapter`](../../modules/301-prometheus-metrics-adapter/)) и прошлое потребление ресурсов (делая запросы в Trickster перед Prometheus) и предоставляет рекомендации по CPU и памяти для контейнеров.
- `Updater` — проверяет, что у подов с VPA выставлены корректные ресурсы, если нет — убивает эти поды, чтобы контроллер пересоздал поды с новыми resource requests.
- `Admission Plugin` — задает resource requests при создании новых подов (контроллером или из-за активности Updater'а).

При изменении ресурсов компонентом Updater это происходит с помощью [Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#the-eviction-api), поэтому учитываются `Pod Disruption Budget` для обновляемых подов.

![Архитектура VPA](https://raw.githubusercontent.com/kubernetes/design-proposals-archive/acc25e14ca83dfda4f66d8cb1f1b491f26e78ffe/autoscaling/images/vpa-architecture.png)
