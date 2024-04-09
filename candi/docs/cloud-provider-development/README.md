Перед началом разработки Deckhouse cloud-provider необходимо определить наличие готовых решений для интеграции с облачным провайдером(Terraform provider, Cloud Controller Manager, Container Storage Interface, Cluster API). Если таковые отсутствуют, то разработчику необходимо реализовать интеграцию самостоятельно.

Пример реализации Deckhouse cloud-provider можно найти в отдельной ветке репозитория Deckhouse: [cloud-provider-sample](https://github.com/deckhouse/deckhouse/tree/cloud-provider-sample).

Новый Deckhouse cloud-provider необходимо добавлять в директорию ee/modules/030-<cloud-provider-name>.

Deckhouse cloud-provider состоит из следующих компонентов:
* [Candi](candi.md)
* [Terraform и Cloud Layouts](terraform.md)
* [Cloud data discoverer](cloud-data-discoverer.md)
* [Cloud Controller Manager](ccm.md)
* [Container Storage Interface module](csi.md)
* [Cluster API Provider](capi.md)

Для корректной работы кластера необходимо включить [CNI модуль](cni.md).

[Checklist](checklist.md) для разработки Deckhouse cloud-provider.
