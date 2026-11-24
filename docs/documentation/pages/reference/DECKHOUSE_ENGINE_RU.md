---
title: Deckhouse Engine
permalink: ru/reference/deckhouse-engine.html
lang: ru
search: Deckhouse Engine, systemType, DeckhouseEngine, NodeConfig, nodelet, неизменяемый узел, dm-verity, гипервизор первого типа
description: Что такое Deckhouse Engine, как устроен узел, как он получает конфигурацию, обновляется и обслуживается.
relatedLinks:
  - title: Установка кластера на bare metal с Deckhouse Engine
    url: ../installing/deckhouse-engine.html
  - title: Архитектура узлов Deckhouse Engine
    url: ../architecture/cluster-and-infrastructure/node-management/deckhouse-engine.html
---

Deckhouse Engine — гипервизор первого типа для узлов Deckhouse Platform. Машина загружается
сразу в Deckhouse Engine, без промежуточной системы общего назначения. Корень узла доступен только для чтения
и каждый раз проверяется. Пакетного менеджера, демона `sshd` и командной оболочки на узле нет.
Узлом управляет агент `nodelet`, который приводит узел к состоянию, описанному в ресурсе
[NodeConfig](/modules/node-manager/cr.html#nodeconfig), и сообщает о результате в его статусе.

Узлы Deckhouse Engine объединяются в группы так же, как остальные узлы кластера. Признаком группы таких узлов является
параметр [`spec.systemType: DeckhouseEngine`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype)
ресурса NodeGroup. Группы с `systemType: GenericOS` или без этого параметра состоят из
узлов, которые настраивает [bashible](../architecture/cluster-and-infrastructure/node-management/bashible.html).

## Отличие узла Deckhouse Engine от узла GenericOS

Тип системы узла определяет, как его настраивать, обновлять и получать к нему доступ.

| | GenericOS | DeckhouseEngine |
|---|---|---|
| Кто настраивает узел | bashible, шагами на узле | `nodelet` по описанию NodeConfig |
| Корень системы | изменяемый, пакеты дистрибутива | образ erofs, проверка dm-verity, только для чтения |
| Компоненты Kubernetes | пакеты registry-packages | системные расширения (systemd-sysext), подписанные |
| Обновление | пакетами, на месте | заменой образа корня, с откатом |
| Доступ на узел | SSH | API кластера, консоль, портал обслуживания |
| [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) | выполняется | не выполняется |

## Обновление

Образ системы обновляется целиком. Когда в NodeConfig меняется дайджест образа, `nodelet`
скачивает новый образ, проверяет его подпись и перезагружает узел с использованием нового образа. Если узел не подтвердил успешную
загрузку, initramfs возвращает загрузку предыдущего образа.

Образы Deckhouse Engine поставляются вместе с Deckhouse Platform как образы модуля [`node-manager`](/modules/node-manager/), и
версия образа соответствует [релизу платформы](../release-notes.html). Версию образа можно как повышать, так и понижать.

## Требования к узлам

Перед установкой Deckhouse Engine проверьте оборудование и сетевой доступ:

- Архитектура x86-64, загрузка через UEFI. Legacy BIOS не поддерживается.
- Системный диск не менее 20 ГиБ.
- Для размещения etcd на отдельном диске необходим пустой диск от 10 ГиБ. В конфигурации с одним диском etcd использует раздел `DATA`.
- Доступ с узла к API кластера и к хранилищу образов контейнеров Deckhouse.
- Сетевой доступ к TCP-порту `50000` машины с хоста установщика или бастиона, через который добавляется узел.
  При установке кластера `dhctl` также обращается к TCP-порту `50001` первого master-узла.

## Диагностика без входа на узел

На узел Deckhouse Engine нельзя войти по SSH. Состояние узла можно отследить:

- в статусе ресурса NodeConfig — условия по каждой подсистеме (монтирование дисков, containerd,
  kubelet, образ и другие) и фаза узла;
- на консоли машины (монитор и последовательный порт) — экран `nodelet` с состоянием узла,
  адресом портала обслуживания и сетевыми интерфейсами и полный журнал на последовательной
  консоли;
- в журналах и файлах узла через отладочный под — `d8 k debug node/<NODE_NAME> -it --image=<IMAGE>`.

Если узел потерял связь с кластером, у него есть портал обслуживания по адресу
`https://<NODE_ADDRESS>:50000`. Вход — по одноразовому токену из `status.maintenanceToken`
ресурса NodeConfig (нужно право на `nodeconfigs/sensitive`). Изменять что-либо через портал
можно, только если узел больше минуты не получает ответа от API кластера: например, временно
поправить сеть или загрузить исправленный NodeConfig.

## Ограничения

Учитывайте следующие ограничения при выборе способа установки и настройки узлов:

- Узел Deckhouse Engine нельзя настроить ресурсом NodeGroupConfiguration — все настройки задаются через NodeConfig.
- `systemType: DeckhouseEngine` нельзя снять с группы или заменить на `GenericOS`. Группа
  может стать `DeckhouseEngine`, только пока в ней нет узлов, настроенных bashible.
- Установка первых master-узлов из бандла (режим `Local` модуля [registry](/modules/registry/))
  не поддерживается. Для закрытого окружения используйте модуль registry в режиме `Managed`
  с хранилищем образов внутри периметра, заполненным через `d8 mirror push`.
