---
title: "Сloud provider — Azure: custom resource"
---

## AzureInstanceClass

Ресурс описывает параметры группы Azure Instances, которые будeт использовать machine-controller-manager из модуля [node-manager](/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции описаны в `.spec`.

* `machineSize` — тип заказываемых instances.
    * Обязательный параметр.
    * Список всех доступных типов в регионе, можно посмотреть с помощью [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).
        * `az vm list-sizes --location westeurope -o table`
* `urn` — образ виртуальной машины, который будет использоваться для instance'ов.
    * Опциональный параметр.
    * Формат — строка `publisher:offer:sku:version`
    * Пример: `Canonical:UbuntuServer:18.04-LTS:18.04.202010140`.
    * Последнюю доступную версию можно получить c помощью [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli).
        * `az vm image show --urn Canonical:UbuntuServer:18.04-LTS:latest --location westeurope`
        * `az vm image show --urn Canonical:0001-com-ubuntu-server-focal:20_04-lts:latest --location westeurope`
    * Подробнее про образы виртуальных машин можно посмотреть в [официальной документации](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage).
    * **Внимание!** Сейчас поддерживается и тестируется только Ubuntu 18.04/Centos 7.
    * По умолчанию используется образ из `AzureCloudDiscoveryData`. Это образ из которого был создан мастер при развертывании кластера.
* `diskType` — тип созданного диска.
    * По умолчанию `StandardSSD_LRS`.
    * Список доступных типов дисков, можно посмотреть с помощью [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
        * `az vm list-skus -l westeurope --zone`
    * Опциональный параметр.
* `diskSizeGb` — размер root диска.
    * Формат — integer. В ГиБ.
    * По умолчанию `50` ГиБ.
    * Опциональный параметр.

#### Пример AzureInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```
