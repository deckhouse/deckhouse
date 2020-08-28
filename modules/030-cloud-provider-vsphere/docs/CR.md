---
title: "Сloud provider — VMware vSphere: Custom Resources"
---

## VsphereInstanceClass

Ресурс описывает параметры группы vSphere VirtualMachines, которые будет использовать machine-controller-manager из модуля [node-manager](/modules/040-node-manager/). На `VsphereInstanceClass` ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `numCPUs` — количество виртуальных процессорных ядер, выделяемых VirtualMachine.
    * Формат — integer
* `memory` — количество памяти, выделенных VirtualMachine.
    * Формат — integer. В мебибайтах.
* `rootDiskSize` — размер корневого диска в VirtualMachine. Если в template диск меньше, автоматически произойдёт его расширение.
    * Формат — integer. В гибибайтах.
* `template` — путь до VirtualMachine Template, который будет склонирован для создания новой VirtualMachine.
    * Пример — `dev/golden_image`
* `mainNetwork` — путь до network, которая будет подключена к виртуальной машине, как основная сеть (шлюз по умолчанию).
    * Пример — `k8s-msk-178`
* `additionalNetworks` — список путей до networks, которые будут подключены к виртуальной машине.
    * Формат — массив строк.
    * Пример:

        ```yaml
        - DEVOPS_49
        - DEVOPS_50
        ```

* `datastore` — путь до Datastore, на котором будет созданы склонированные виртуальные машины.
    * Пример — `lun-1201`
* `resourcePool` — путь до Resource Pool, в котором будут созданые склонированные виртуальные машины.
    * Пример — `prod`
    * Опциональный параметр.
* `resourcePoolForNewNodes` — полный аналог опции `resourcePool`, при изменении параметра **не происходит** перекат нод.
    * Пример — `prod`
    * Опциональный параметр.
* `runtimeOptions` — опциональные параметры виртуальных машин.
    * `nestedHardwareVirtualization` — включить [Hardware Assisted Virtualization](https://docs.vmware.com/en/VMware-vSphere/6.5/com.vmware.vsphere.vm_admin.doc/GUID-2A98801C-68E8-47AF-99ED-00C63E4857F6.html) на созданных виртуальных машинах
        * Формат — bool.
        * Опциональный параметр.
    * `cpuShares` — относительная величина CPU Shares для создаваемых виртуальных машин.
        * Формат — integer.
        * Опциональный параметр.
        * По умолчанию, `1000` на каждый vCPU.
    * `cpuLimit` — Верхний лимит потребляемой частоты процессоров для создаваемых виртуальных машин.
        * Формат — integer. В MHz.
        * Опциональный параметр.
    * `cpuReservation` — величина зарезервированный для виртуальной машины частоты CPU.
        * Формат — integer. В MHz.
        * Опциональный параметр.
    * `memoryShares` — относительная величина Memory Shares для создаваемых виртуальных машин.
        * Формат — integer. От 0 до 100.
        * Опциональный параметр.
        * По умолчанию, `10` shares на мегабайт.
    * `memoryLimit` — Верхний лимит потребляемой памяти для создаваемых виртуальных машин.
        * Формат — integer. В MB.
        * Опциональный параметр.
    * `memoryReservation` — процент зарезервированный для виртуальной машины памяти в кластере. В процентах относительно `.spec.memory`.
        * Формат — integer. От 0 до 100.
        * Опциональный параметр.
        * По умолчанию, `80`.

### Пример

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  rootDiskSize: 20
  template: dev/golden_image
  network: k8s-msk-178
  datastore: lun-1201
```
