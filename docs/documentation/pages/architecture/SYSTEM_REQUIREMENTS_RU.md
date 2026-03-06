---
title: Системные требования
permalink: ru/architecture/system-requirements/
lang: ru
search: system requirements, системные требования
---

Deckhouse Kubernetes Platform (DKP) может устанавливаться в следующих вариантах:

* **В поддерживаемом облаке**, включая [публичные](/products/kubernetes-platform/documentation/v1/admin/integrations/public/overview.html) и [частные облака](/products/kubernetes-platform/documentation/v1/admin/integrations/private/overview.html), а также [системы виртуализации](/products/kubernetes-platform/documentation/v1/admin/integrations/virtualization/overview.html). Установщик автоматически создает и настраивает все необходимые ресурсы (включая виртуальные машины, сетевые объекты и т.д.), разворачивает кластер Kubernetes и устанавливает DKP. Для каждого облачного провайдера и системы виртуализации необходимо соблюсти ряд требований. С полным списком требований для каждого варианта интеграции с **IaaS** можно ознакомиться в разделе [Интеграция с IaaS](/products/kubernetes-platform/documentation/v1/admin/integrations/integrations-overview.html) документации.

* **На серверах bare-metal (в том числе гибридные кластеры) или в неподдерживаемых облаках**. Установщик выполняет настройку указанных в конфигурации серверов или виртуальных машин, разворачивает кластер Kubernetes и устанавливает DKP. Системные требования к серверам, используемым для развертывания платформы DKP, зависят от [сценариев развертывания](/products/kubernetes-platform/guides/hardware-requirements.html#%D1%81%D1%86%D0%B5%D0%BD%D0%B0%D1%80%D0%B8%D0%B8-%D1%80%D0%B0%D0%B7%D0%B2%D1%91%D1%80%D1%82%D1%8B%D0%B2%D0%B0%D0%BD%D0%B8%D1%8F). Подробнее с оценкой ресурсов, необходимых для установки DKP, вы можете ознакомиться в следующих руководствах:

  * [Руководство по подбору ресурсов для кластера на bare metal](/products/kubernetes-platform/guides/hardware-requirements.html)
  * [Руководство разметке и объему дисков](/products/kubernetes-platform/guides/fs-requirements.html)
  * [Руководство по подготовке к production](/products/kubernetes-platform/guides/production.html)

* **В существующем кластере Kubernetes**. Установщик разворачивает DKP и интегрирует его с текущей инфраструктурой. Для оценки ресурсов существующего кластера, необходимых для функционирования платформы, можно воспользоваться руководствами для bare-metal серверов, упомянутыми выше. Кластер Kubernetes, в котором устанавливается DKP, должен быть версии из [списка поддерживаемых](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes).

Перед установкой необходимо убедиться в следующем:

* Для кластера на bare-metal (в том числе гибридного кластера) и при установке в неподдерживаемых облаках: сервер использует операционную систему из [списка поддерживаемых ОС](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html) или совместимую с ним, а также доступен по SSH через ключ.

* При настройке интеграции с поддерживаемыми облаками: имеются необходимые квоты для создания ресурсов и подготовлены параметры доступа к облачной инфраструктуре (зависят от конкретного провайдера).

* Есть доступ к хранилищу образов контейнеров Deckhouse (публичный — `registry.deckhouse.io` или `registry.deckhouse.ru`, либо зеркало).
