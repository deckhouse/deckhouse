---
title: "Prometheus-мониторинг: настройки"
type:
  - instruction
search: prometheus
---

Включен по умолчанию и не требует обязательной конфигурации (всё работает из коробки).

## Параметры

<!-- SCHEMA -->

## Примечание

* `retentionSize` для `main` и `longterm` **рассчитывается автоматически, возможности задать значение нет!**
  * Алгоритм расчета:
    * `pvc_size * 0.8` — если PVC существует.
    * `10 GiB` — если PVC нет и StorageClass поддерживает ресайз.
    * `25 GiB` — если PVC нет и StorageClass не поддерживает ресайз.
  * Если используется `local-storage` и требуется изменить `retentionSize`, то необходимо вручную изменить размер PV и PVC в нужную сторону. **Внимание!** Для расчета берется значение из `.status.capacity.storage` PVC, поскольку оно отражает рельный размер PV в случае ручного ресайза.
* Размер дисков prometheus можно изменить стандартным для kubernetes способом (если в StorageClass это разрешено), отредактировав в PersistentVolumeClaim поле `.spec.resources.requests.storage`.
