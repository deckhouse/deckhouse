# Место для примеров

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Для этого необходимо перейти в **Settings** -> **Developer settings** -> **OAuth Aps** -> **Register a new OAuth application** и в качестве **Authorization callback URL** указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

--------------

При disruption update выполняется evict подов с узла. Если какие-либо поды не удалось evict'нуть, evict повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После этого поды, которые не удалось evict'нуть, удаляются.

При disruption-обновлении кластера происходит вытеснение (evict) пода на узле. Если некоторые поды не удается вытеснить, это действие повторяется каждые 20 секунд, пока не наступит глобальный тайм-аут в 5 минут. После чего, поды, которые не удалось вытеснить, удаляются.

--------------

Определяет допустимые к использованию значения `runAsGroup`.

Задает основные группы (`runAsGroup`), разрешенные для использования в параметре `securityContext`.

--------------

В случае изменения параметров `InstanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит. Количество заказываемых одновременно `MachineDeployment` определяется параметром `cloudInstances.maxSurgePerZone`.

В случае изменения параметров `instanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить плавающее обновление (rolling update). Deckhouse создаст новые MachineDeployment, а старые удалит. Количество заказываемых одновременно MachineDeployment определяется параметром `cloudInstances.maxSurgePerZone`.
