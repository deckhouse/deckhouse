# <cloud-provider-name>ControlPlane Controller

Контроллер <cloud-provider-name>ControlPlane реализует только небольшое подмножество функциональных возможностей контроллера `CAPI Control Plane`, требуемых `CAPI Machine Controller`.

Основные обязанности контроллера <cloud-provider-name>ControlPlane:

* Управление жизненным циклом объекта <cloud-provider-name>ControlPlane, на который ссылается `Cluster.spec.controlPlaneRef`.
* Установка поля `status.initialized`.
* Установка поля `status.ready`.
* Установка поля `status.externalManagedControlPlane`.

По соглашению объект `<cloud-provider-name>ControlPlane` **должен** иметь объект `status`.

Объект `status` имеет следующие определенные поля:

* `initialized` — логическое поле, которое принимает значение `true`, когда кластер
  завершил инициализацию. Всегда имеет значение `true`.
* `ready` - логическое поле, обозначает что API-сервер готов принимать запросы. Всегда устанавливается в значение `true`.
* `externalManagedControlPlane` - это логическое значение, которое должно быть установлено в значение `true`, если объекты Node не
  существуют в кластере. Важно скрыть объекты Node, которые не связаны с объектами <cloud-provider-name>Instance.
  Всегда устанавливается в значение `true`.
