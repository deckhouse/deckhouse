# Container Network Interface

Конфигурирование модуля CNI происходит во время бутстрапа кластера в `dhctl` и при помощи ресурса ModuleConfig(`cni-cilium`, `cni-flannel`, `cni-simple-bridge`). Если ModuleConfig не указан, то по умолчанию `dhctl` включает модуль `cni-cilium`.

Для нового Deckhouse cloud-provider необходимо сконфигурировать модуль CNI с настройками по умолчанию в `dhctl`. Пример можно найти в `cloud-provider-sample`.
