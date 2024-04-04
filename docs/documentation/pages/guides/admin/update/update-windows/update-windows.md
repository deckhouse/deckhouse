## Окна обновлений

Управление [окнами обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы Deckhouse в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

**Временные настройки окон**

Временные настройки позволяют определить удобное время для обновления модулей в Deckhouse Kubernetes Platform. Это позволbт обеспечить стабильность системы во время обновлений и минимизировать возможные негативные влияния на работающие приложения.
Установка обновлений в определенное время позволяет минимизировать возможные проблемы, связанные с нагрузкой на систему во время установки обновлений, а также предотвращает возможные конфликты между обновляемыми модулями и работающими приложениями.

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```