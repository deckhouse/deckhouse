# <cloud-provider-name>Cluster Controller

Контроллер <cloud-provider-name>Cluster реализует только небольшое подмножество функций `CAPI Infrastructure Provider`, требуемых `CAPI Cluster Controller`.

Основные обязанности контроллера <cloud-provider-name>Cluster:

* Управление жизненным циклом объекта <cloud-provider-name>Cluster, на который ссылается `Cluster.spec.infrastructureRef`.
* Настройка полей `spec.controlPlaneEndpoint` и `status.ready`.

По соглашению объект `<cloud-provider-name>Cluster` **должен** иметь объекты `spec` и `status`.

Объект `spec` имеет следующие поля:

- `controlPlaneEndpoint` - определяет endpoint, используемый для подключения к API серверу кластера.

Объект `status` имеет следующие определенные поля:

- `ready` - логическое поле, которое имеет значение `true`, когда инфраструктура готова к использованию. Всегда имеет значение `true`.
