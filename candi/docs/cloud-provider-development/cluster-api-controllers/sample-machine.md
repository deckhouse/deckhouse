# <cloud-provider-name>Machine Controller

Основные обязанности контроллера <cloud-provider-name>Machine:

* Управление жизненным циклом объектов <cloud-provider-name>Machine.
* Установка поля `<cloud-provider-name>Machine.spec.providerID`.

По соглашению объект `<cloud-provider-name>Machine` **должен** иметь объекты `spec` и `status`.

Объект `spec` имеет следующие определенные поля:

* `providerID` — идентификатор провайдера, идентифицирующий машину.

Объект `status` имеет следующие определенные поля:

* `ready` — логическое поле, указывающее, готова ли инфраструктура к использованию или нет.
* `failureReason` - строка, которая объясняет, почему произошла фатальная ошибка.
* `failureMessage` - строка, которая содержит сообщение, содержащееся в ошибке.
* `addresses` - `MachineAddresses` (список `MachineAddress`), который представляет имена хостов, внешние IP-адреса, внутренние IP-адреса,
  внешние DNS-имена и/или внутренние DNS-имена для экземпляра машины провайдера. `MachineAddress` определяется как:
- `type` (строка): один из `Hostname`, `ExternalIP`, `InternalIP`, `ExternalDNS`, `InternalDNS`
- `address` (строка)
