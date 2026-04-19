В этом разделе рассматривается настройка интеграции Deckhouse Kubernetes Platform (DKP) с облачными ресурсами в частном облаке на базе [{{ site.data.admin.cloud-types.types[page.cloud_type].name }}]({{ site.data.admin.cloud-types.types[page.cloud_type].link }}).

Интеграция даёт возможность использовать ресурсы облака на базе {{ site.data.admin.cloud-types.types[page.cloud_type].name }} при заказе узлов для заданной [группы узлов](../../../configuration/platform-scaling/node-management.html#конфигурация-группы-узлов).

Основные возможности:

- Управление ресурсами {{ site.data.admin.cloud-types.types[page.cloud_type].name }}:
  - актуализация метаданных Servers и Kubernetes Nodes;
  - удаление из кластера узлов, которых уже нет в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}.
- Заказ дисков в Cinder (block) {{ site.data.admin.cloud-types.types[page.cloud_type].name }} с помощью компонента `CSI storage` (Manilla (filesystem) пока не поддерживается).
