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

## 006-recommendation-factor.patch

Позволяет умножать рекомендацию на множитель, который задаётся в политике ресурсов контейнера отдельно для каждого ресурса.

- Добавляет в `ContainerResourcePolicy` поле `recommendationFactor`: множитель для каждого ресурса с ключами `cpu` и `memory`. Значения задаются строками Quantity, например `"1.2"`. Значение должно быть больше 0 и не больше 10, с не более чем двумя знаками после запятой. Отсутствующий ключ означает 1; webhook отклоняет любые другие имена ресурсов.
- Множитель применяется в estimator chain компонента recommender: после safety margin и confidence-множителей, но до минимума для пода, округления и ограничения по политике (`minAllowed`, `maxAllowed`). Он одинаково масштабирует `target`, `lowerBound` и `upperBound`.
- `uncappedTarget` сохраняет значение, посчитанное без множителя (минимум для пода и округление при этом применяются). Метрики качества и `lastRecommendation` по-прежнему считаются по `uncappedTarget`, поэтому множитель на них не влияет.
- Невалидное значение, которое уже сохранено в объекте (такое возможно, только если webhook был обойдён), recommender игнорирует и пишет строку в лог, а множитель этого ресурса остаётся равным 1.
- Webhook и recommender используют общую функцию `Parse` из `pkg/utils/recommendationfactor`. Она проверяет диапазон и число знаков после запятой по точному значению Quantity, потому что `MilliValue()` округляет вверх значения меньше `1m` и переполняет `int64`; перед этим проверка порядка величины по приближению `float64` отсекает экстремальные показатели степени, например `1e1000000000`, за время, не зависящее от показателя. Каждая её ошибка несёт короткое представление отклонённого значения (`ValueOf`), и именно оно попадает в сообщения об ошибках и в логи.
- `isNoopContainerPolicy` из патча 004 считает политику контейнера с `recommendationFactor` настоящей политикой, поэтому для неё вызывается `UpdateFromPolicy`, и множитель попадает в `AggregateContainerState`, а политика не пропускается как no-op.
- `deploy/vpa-v1-crd-gen.yaml` получает новое поле и обновлённое описание `uncappedTarget`, чтобы исходники оставались согласованными с Go-типами. Собственный CRD модуль устанавливает из `crds/`; он поддерживается отдельно.
