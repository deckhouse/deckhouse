---
title: Нет доступа по серийной консоли, но VNC работает — в чём причина?
section: vm_operations
lang: ru
---

К ВМ можно подключиться через серийную консоль ([`d8 v console`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-console)) или по VNC ([`d8 v vnc`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-vnc)). Способы используют разные каналы связи с гостевой ОС и зависят от её настройки.

Серийная консоль подключается к порту `ttyS0` в гостевой ОС. Если служба `getty` для этого порта не запущена, при подключении через `d8 v console` не появится приглашение ко входу, хотя VNC продолжит работать.

В гостевой ОС включите и запустите службу `serial-getty` для `ttyS0`:

```bash
sudo systemctl enable --now serial-getty@ttyS0.service
```

После этого снова подключитесь к серийной консоли.
