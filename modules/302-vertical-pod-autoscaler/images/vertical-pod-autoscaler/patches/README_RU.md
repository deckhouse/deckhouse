# Patches

## 002-openkruise-daemonset-apiversion.patch

Этот патч для корректной работы VPA с OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1) в Дэкхаусе

## 003-recommender.patch

Этот патч не работает для хранилища Prometheus. Только для контрольных точек VPA.
Не имею представления, для чего он нужен.
Поскольку мы используем хранилище Prometheus, не буду перемещать этот патч.

## 004-updater.patch
1. Модуль
   vertical-pod-autoscaler — компонент Updater, пакет pkg/updater/restriction: фабрика ограничений (pods_restriction_factory.go), ограничение эвиктов (pods_eviction_restriction.go), ограничение in-place обновлений (pods_inplace_restriction.go) и связанные тесты.
2. Что делает патч
   В singleGroupStats добавлено поле belowMinReplicas (bool): группа учитывается в картах даже при livePods < minReplicas, но помечается как «ниже minReplicas».
   В GetCreatorMaps при actual < required группа больше не пропускается (continue убран): для неё по-прежнему считаются configured, running, evictionTolerance и т.д., в карты она попадает с belowMinReplicas = true.
   В PodsEvictionRestrictionImpl.CanEvict в начале добавлена проверка: если у группы belowMinReplicas == true, возвращается false и пишется лог — эвикт (recreate) для таких групп запрещён.
   PodsInPlaceRestriction флаг belowMinReplicas не учитывает: решение о in-place по-прежнему строится на isPodDisruptable(), без проверки minReplicas. In-place разрешён и при belowMinReplicas == true.
   Обновлены/добавлены тесты: проверка in-place для синглтона при belowMinReplicas, лимит по tolerance в одном цикле, блокировка fallback на eviction при belowMinReplicas (TestFallbackToRecreateBlockedWhenBelowMinReplicas).
   Итог: при InPlaceOrRecreate группы ниже minReplicas могут получать только in-place обновления; eviction/recreate для них запрещён. Если in-place невозможен (например, Infeasible), fallback на eviction не выполняется — под не эвиктится, только логирование.
3. Цель изменений
   Сделать так, чтобы при updateMode: InPlaceOrRecreate рекомендации VPA могли применяться и к группам с числом реплик ниже minReplicas (в т.ч. к одному поду), но только через in-place resize, без эвикта и пересоздания. Если in-place применить нельзя — не делать eviction (ничего не делать и логировать), чтобы не снижать доступность.
4. Какую проблему решает
   Раньше при livePods < minReplicas (например, один под при дефолтном --min-replicas=2) группа вообще не попадала в карты в GetCreatorMaps (из-за if actual < required { continue }). В результате:
   In-place для таких подов был недоступен (под не находился в podToReplicaCreatorMap / не было статистики группы).
   Пользователи с одной репликой или с малым числом реплик не могли получить применение рекомендаций VPA в режиме InPlaceOrRecreate без повышения minReplicas или числа реплик.
   Патч разделяет две вещи:
   Защита от эвикта при низком числе реплик — по-прежнему обеспечивается (eviction блокируется по belowMinReplicas).
   Разрешение in-place при любом числе реплик — теперь возможно: группы ниже minReplicas включаются в карты с флагом, in-place для них разрешён, eviction — нет. Если in-place невозможен, fallback на recreate не приводит к evict — проблема «нельзя применить рекомендации к синглтону / малой группе» снимается без риска лишних эвиктов.
