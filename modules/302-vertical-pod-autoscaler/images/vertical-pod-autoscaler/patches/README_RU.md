# Patches

## 002-openkruise-daemonset-apiversion.patch

Этот патч для корректной работы VPA с OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1) в Дэкхаусе

## 003-recommender.patch

Этот патч не работает для хранилища Prometheus. Только для контрольных точек VPA.
Не имею представления, для чего он нужен.
Поскольку мы используем хранилище Prometheus, не буду перемещать этот патч.

## 004-daemonset-scope-node-label.patch

Добавляет scoped-рекомендации для DaemonSet с группировкой по ключу лейбла узла из `spec.scope`.

- Работает только для `targetRef.kind=DaemonSet` при непустом `spec.scope`.
- Использует значение лейбла узла как ключ группы рекомендаций для потока с Prometheus.
- Использует специальное значение `scopeValue="__absent__"`, если на узле нет ключа лейбла из `spec.scope`.
- Использует `status.groups` как источник истины для scoped-рекомендаций.
- Хранит grouped-рекомендации в compact-виде (`containerName` + `target`).
- Не дублирует `scope` внутри grouped `containerRecommendations`; consumer'ы вычисляют его из `spec.scope` + `group.scopeValue`.
- Делает поля взаимоисключающими:
  - `status.recommendation` используется только для обычных VPA (включая DaemonSet без `spec.scope`);
  - `status.groups` используется только для scoped DaemonSet, а `status.recommendation` не заполняется.
- Admission-controller/updater читают только актуальное поле статуса в зависимости от режима VPA.

## 005-default-update-mode-in-place-or-recreate.patch

Позволяет модулю сделать режимом обновления по умолчанию `InPlaceOrRecreate` вместо upstream-значения `Recreate`, как и указано в документации модуля.

- Режим по умолчанию задаёт переменная окружения `VPA_DEFAULT_UPDATE_MODE`. Изменить его может только значение `InPlaceOrRecreate`. Без переменной или с любым другим значением режимом по умолчанию остаётся `Recreate`, как в upstream.
- Шаблоны модуля задают переменную для updater и admission controller: `InPlaceOrRecreate` в Kubernetes 1.33 и новее, где есть in-place resize, и `Recreate` в остальных случаях.
- `GetUpdateMode` возвращает этот режим, если `spec.updatePolicy.updateMode` не задан, а admission webhook записывает его в VPA без `spec.updatePolicy`.
- Updater получает режим через `GetUpdateMode`, когда отбирает поды для in-place-обновления. В upstream там прямое разыменование `spec.updatePolicy.updateMode`, поэтому с новым режимом по умолчанию VPA без `updatePolicy` приводил бы к panic в updater.
- Явно заданные режимы, включая `Recreate` и устаревший `Auto`, не меняются. VPA, которым webhook уже записал режим, сохраняют его.
