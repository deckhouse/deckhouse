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

```
dhctl bootstrap-phase base-infra --config config
```

* Поднять пиринг по инструкции [выше](#как-поднять-пиринг-между-vpc).
* Продолжить установку кластера, на вопрос про кеш терраформа нужно ответить "y":

```
dhctl bootstrap --config config --ssh-...

```

## Как создать кластер в новом VPC и развернуть bastion для доступа к узлам?

* Выполнить бутстрап base-infrastructure кластера:

```
dhctl bootstrap-phase base-infra --config config
```

* Запустить вручную bastion в subnet <prefix>-public-0.
* Продолжить установку кластера, на вопрос про кеш терраформа нужно ответить "y":

```
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
