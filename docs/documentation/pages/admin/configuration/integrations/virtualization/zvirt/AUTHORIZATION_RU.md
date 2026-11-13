---
title: Подключение и авторизация в zVirt
permalink: ru/admin/integrations/virtualization/zvirt/authorization.html
lang: ru
---

Чтобы Deckhouse Platform могла создавать виртуальные машины в zVirt, назначьте пользователю zVirt необходимые роли, подготовьте cloud-образ операционной системы и шаблон виртуальной машины, а логин и пароль пользователя передайте в секрете с учётными данными.

## Требования

Для работы Deckhouse Platform с zVirt необходимы:

- рабочая инсталляция zVirt с доступным API;
- zVirt версии `4.0`–`4.4`;
- пользователь [с правами из раздела «Права пользователя»](#права-пользователя);
- домен хранения с загруженным cloud-образом операционной системы;
- шаблон виртуальной машины, подготовленный на основе cloud-образа.

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

## Права пользователя

Deckhouse Platform не требует для пользователя zVirt роли `SuperUser`. Достаточно назначить пользователю следующие предопределённые роли zVirt на объекты, с которыми работает кластер:

| Роль | Объект | Назначение |
| --- | --- | --- |
| `UserVmManager` | Кластер zVirt из параметра [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid) | Создание, запуск, остановка и удаление виртуальных машин узлов, создание их сетевых интерфейсов, подключение и отключение дисков |
| `UserTemplateBasedVm` | Шаблон виртуальной машины | Создание виртуальных машин из шаблона |
| `DiskOperator` | Домены хранения, в которых размещён диск шаблона и создаются диски для etcd и тома кластера | Создание, изменение размера, подключение, отключение и удаление дисков |
| `VnicProfileUser` | Профиль vNIC | Создание сетевого интерфейса виртуальной машины на основе профиля vNIC |
| `TagManager` | Вся система (системное разрешение) | Создание тега кластера и назначение его виртуальным машинам CloudEphemeral-узлов |

Роль `TagManager` назначается только на всю систему, так как zVirt проверяет право на управление тегами на этом уровне.

{% alert level="warning" %}
Роль `TagManager` административная. С ней пользователь может просматривать через API zVirt все объекты, но изменять может только объекты, на которые ему назначены роли из таблицы.
{% endalert %}

Некоторые роли zVirt по умолчанию назначает группе Everyone, и их можно не назначать пользователю отдельно:

- `UserTemplateBasedVm` на шаблон, при создании которого установлен флажок «Разрешить всем пользователям доступ к шаблону»;
- `VnicProfileUser` на публичный профиль vNIC;
- `CpuProfileOperator` на профиль ЦП кластера;
- `DiskProfileUser` на профили дисков доменов хранения.

Если в вашей инсталляции zVirt эти разрешения удалены, назначьте роли пользователю.

Чтобы назначить роль на объект:

1. Откройте объект в портале администрирования zVirt, например кластер в разделе «Ресурсы → Кластеры».
1. Перейдите на вкладку «Разрешения» и нажмите «Добавить».
1. Выберите пользователя и роль и сохраните разрешение.

Роль `TagManager` назначьте в разделе «Управление → Настройка» на вкладке «Системные разрешения».

## Подготовка cloud-образа

Cloud-образ операционной системы (ОС) загружается в хранилище дисков zVirt. Чтобы загрузить образ:

1. Откройте портал администрирования zVirt и перейдите в раздел «Хранилище → Диски».
1. Загрузите cloud-образ ОС (например, `.qcow2` или `.img`) в хранилище.

   ![Начало загрузки cloud-образа ОС](../../../../images/cloud-provider-zvirt/template/disk-upload-menu-ru.png)

   ![Процесс загрузки cloud-образа ОС](../../../../images/cloud-provider-zvirt/template/disk-upload-dialog-ru.png)

1. Дождитесь окончания загрузки. Диск должен получить статус `OK`.

   ![Окончание загрузки cloud-образа ОС](../../../../images/cloud-provider-zvirt/template/disk-upload-status-ru.png)

## Подготовка шаблона виртуальной машины

Шаблон создаётся на основе виртуальной машины (ВМ), к которой прикреплён cloud-образ ОС. Чтобы подготовить шаблон:

1. Откройте раздел «Ресурсы → Виртуальные машины» и создайте ВМ.
1. Укажите следующие параметры:

   - «Шаблон» — `Blank`;
   - «Операционная система» — ОС из загруженного cloud-образа;
   - «Профиль нагрузки» — «Высокая производительность»;
   - «Виртуальные диски» — нажмите «Прикрепить», выберите загруженный cloud-образ, установите флажок в столбце «OS», чтобы сделать диск загрузочным, и выберите интерфейс «VirtIO».

   ![Настройка общих параметров ВМ](../../../../images/cloud-provider-zvirt/template/vm-attach-disk-ru.png)

1. Не добавляйте сетевые интерфейсы. Deckhouse Platform создаёт сетевой интерфейс при создании ВМ для узла.

   ![Пропуск настройки сетевых интерфейсов](../../../../images/cloud-provider-zvirt/template/vm-network-interfaces-ru.png)

1. Остальные параметры оставьте по умолчанию и создайте ВМ.
1. После создания ВМ выберите её в списке, откройте меню дополнительных действий (⋮) и выберите «Создать шаблон».

   ![Создание шаблона из ВМ](../../../../images/cloud-provider-zvirt/template/vm-make-template-menu-ru.png)

1. Укажите имя шаблона и включите параметр «Запечатать шаблон (только Linux)».

   ![Параметры шаблона](../../../../images/cloud-provider-zvirt/template/template-dialog-ru.png)

1. Убедитесь, что шаблон отображается в разделе «Ресурсы → Шаблоны».

   ![Проверка создания шаблона](../../../../images/cloud-provider-zvirt/template/template-list-ru.png)

## Замена образа ОС

Параметр [`template`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-template) ресурса ZvirtInstanceClass задаёт образ узлов типа `CloudEphemeral`. При настройке провайдера через ModuleConfig этот параметр также задаёт образ узлов типа `CloudPermanent` и master-узлов.

При настройке через ZvirtClusterConfiguration образ узлов типа `CloudPermanent` и master-узлов задаётся параметром `template` в секциях `nodeGroups[].instanceClass` и `masterNodeGroup.instanceClass` соответственно.

Порядок замены описан в разделе [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).

## Секрет с учётными данными

Логин и пароль пользователя zVirt хранятся не в [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig), а в отдельном секрете. Модуль [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) читает его и передаёт учётные данные своим компонентам, которые обращаются к API zVirt.

Секрет создаётся при установке кластера вместе с остальными ресурсами первичной конфигурации. Секрет должен отвечать следующим требованиям:

- имя `d8-credentials`, неймспейс `d8-cloud-provider-zvirt`;
- тип `cloud-provider.deckhouse.io/credentials`;
- поле `authScheme` со значением `userPassword`;
- поле `identity` с логином пользователя zVirt;
- поле `secret` с паролем пользователя zVirt.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-zvirt
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: userPassword
  identity: <USERNAME>
  secret: <PASSWORD>
```

Замените `<USERNAME>` и `<PASSWORD>` на логин и пароль пользователя zVirt.

Секрет, который не отвечает этим требованиям, не проходит проверку ни при установке кластера, ни при применении в работающем кластере.

Чтобы сменить учётные данные, обновите поля `identity` и `secret`:

```shell
d8 k -n d8-cloud-provider-zvirt edit secret d8-credentials
```

В кластерах, переведённых на ModuleConfig, логин и пароль переносятся в этот секрет из параметров [`provider.username`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-provider-username) и [`provider.password`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-provider-password) ресурса [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration).
