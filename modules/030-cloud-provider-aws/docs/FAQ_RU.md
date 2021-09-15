---
title: "Cloud provider — AWS: FAQ"
---


## Как поднять пиринг между VPC?

Для примера будем поднимать пиринг между двумя VPC — vpc-a и vpc-b.

**Важно!**
IPv4 CIDR у обоих VPC должен различаться.

* Перейти в регион, где работает vpc-a.
* VPC -> VPC Peering Connections -> Create Peering Connection, настроить пиринг:

  * Name: vpc-a-vpc-b
  * Заполнить Local и Another VPC.

* Перейти в регион, где работает vpc-b.
* VPC -> VPC Peering Connections.
* Выделить свежеиспечённый пиринг и выбрать Action "Accept Request".
* Для vpc-a добавить во все таблицы маршрутизации маршруты до CIDR vpc-b через пиринг.
* Для vpc-b добавить во все таблицы маршрутизации маршруты до CIDR vpc-a через пиринг.


## Как создать кластер в новом VPC с доступом через имеющийся бастион?

* Выполнить бутстрап base-infrastructure кластера:

  ```shell
  dhctl bootstrap-phase base-infra --config config
  ```

* Поднять пиринг по инструкции [выше](#как-поднять-пиринг-между-vpc).
* Продолжить установку кластера, на вопрос про кеш терраформа нужно ответить "y":

  ```shell
  dhctl bootstrap --config config --ssh-...
  ```

## Как создать кластер в новом VPC и развернуть bastion для доступа к узлам?

* Выполнить бутстрап base-infrastructure кластера:

  ```shell
  dhctl bootstrap-phase base-infra --config config
  ```

* Запустить вручную bastion в subnet <prefix>-public-0.
* Продолжить установку кластера, на вопрос про кеш терраформа нужно ответить "y":

  ```shell
  dhctl bootstrap --config config --ssh-...
  ```

## Особенности настройки bastion

Поддерживаются сценарии:
* bastion уже создан во внешней VPC.
  * Создать базовую инфраструктуру — `dhctl bootstrap-phase base-infra`.
  * Настроить пиринг между внешней и свежесозданной VPC.
  * Продолжить инсталляцию с указанием бастиона — `dhctl bootstrap --ssh-bastion...`
* bastion требуется поставить в свежесозданной VPC.
  * Создать базовую инфраструктуру — `dhctl bootstrap-phase base-infra`.
  * Запустить вручную bastion в subnet <prefix>-public-0.
  * Продолжить инсталляцию с указанием bastion — `dhctl bootstrap --ssh-bastion...`

## Добавление CloudStatic узлов в кластер

Для добавления инстанса в кластер требуется:
  * Прикрепить группу безопасности `<prefix>-node`
  * Прописать теги (чтобы cloud-controller-manager мог найти инстансы в облаке):

  ```
  "kubernetes.io/cluster/<cluster_uuid>" = "shared"
  "kubernetes.io/cluster/<prefix>" = "shared"
  ```

  * Узнать `cluster_uuid` можно с помощью команды:

    ```shell
    kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
    ```

  * Узнать `prefix` можно с помощью команды:
    ```shell
    kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
    ```

## Как увеличить размер volume в кластере?

* Задать новый размер в соответствующем ресурсе PersistentVolumeClaim, в параметре `spec.resources.requests.storage`.
* За ходом процесса можно наблюдать в events через команду `kubectl describe pvc`.
* Операция проходит полностью автоматически, и занимает до одной минуты. Никаких дополнительных действий не требуется. 

> ℹ️ После изменения volume вы должны подождать не менее шести часов и убедиться, что том находится в состоянии `in-use` или `available`, прежде чем вы сможете изменить тот же volume еще раз. Подробности вы можете найти в [официальной документации](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/modify-volume-requirements.html).
