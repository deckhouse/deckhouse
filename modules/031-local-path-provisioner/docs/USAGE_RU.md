---
title: "Модуль local-path-provisioner: примеры конфигурации"
---

## Пример CR `LocalPathProvisioner`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

Примечания:

- Этот пример создаст `localpath-system` класс (`storage class`) который **должен** быть использован в pod'ах что бы все заработало
- Все создаваемые хранилища будут иметь политику очистки `Delete` ([issue](https://github.com/deckhouse/deckhouse/issues/360))
- Если этот объект будет удален раньше чем объекты его использующие, папки с сервера удалены не будут
- Обратите внимание - в примере предпологается создание папок на системных нодах, которые скорее всего имеют ряд ограничителей (taints), а как следствие pod'ы **должны** иметь соотв. tolerations
