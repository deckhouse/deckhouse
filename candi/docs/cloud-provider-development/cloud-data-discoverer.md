# Cloud data discoverer

Cloud data discoverer — это компонент Deckhouse cloud-provider, который отвечает за сбор данных из API облачного провайдера и предоставление их в виде ресурса `Secret kube-system/d8-cloud-provider-discovery-data`. Этот ресурс используется другими компонентами Deckhouse cloud-provider.

Пример реализации Cloud data discoverer можно найти в отдельной ветке репозитория Deckhouse - [cloud-provider-sample](https://github.com/deckhouse/deckhouse/tree/cloud-provider-sample/ee/modules/030-cloud-provider-sample).
