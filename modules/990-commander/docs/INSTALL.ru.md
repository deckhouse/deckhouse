---
title: "Установка"
---


## Системные требования

Чтобы начать пользоваться Deckhouse Commander, необходим кластер на базе Deckhouse Kubernetes
Platform.

Мы рекомендуем создать отказоустойчивый управляющий кластер, в котором будут следующие наборы узлов
([NodeGroup](/modules/node-manager/cr.html#nodegroup)):

| Группа узлов | Кол-во узлов | ЦП, ядер | Память, Гб | Диск, Гб |
| ------------ | -----------: | -------: | ---------: | -------: |
| master       |            3 |        4 |          8 |       50 |
| system       |            2 |        4 |          8 |       50 |
| frontend     |            2 |        4 |          8 |       50 |
| commander    |            3 |        8 |         12 |       50 |

Расчет узлов в группе `commander` основан на минимальных требованиях компонентов Deckhouse Commander:

* PostgreSQL в режиме
  HighAvailability
  в двух репликах требует 1 ядро и 1Гб памяти на 2 отдельных узлах (если используете [operator-postgres](#вариант-2-модуль-operator-postgres))
* Сервер API в режиме
  [HighAvailability](/reference/api/global.html#parameters-highavailability)
  в двух репликах требует 1 ядро и 1Гб памяти на 2 отдельных узлах
* Служебные компоненты для рендеринга конфигурации и подключения к прикладными кластерам, требуют
  0.5 ядра и 128 Мб памяти на кластер
* Менеджер кластеров и сервер dhctl совместно требуют ресурсы в зависимости от количества
  обслуживаемых кластеров и одновременно обслуживаемых версий Deckhouse Platform Certified Security Edition
* До 2 ядер на узле могут быть заняты служебными компонентами Deckhouse Platform Certified Security Edition (например: runtime-audit-engine,
  istio, cilium, log-shipper), дополнительно учтен запас памяти.

Системные требования к группе узлов `commander` а так же к конфигурации самих узлов (ядра ЦП и объем
оперативной памяти) варьируются в зависимости от количества кластеров, которые будут обслуживаться
в Deckhouse Commander:

| Кол-во кластеров | ЦП, ядер | Память, Гб | Кол-во узлов 8/8 | Кол-во узлов 8/12 |
| ---------------- | -------: | ---------: | ---------------: | ----------------: |
| 10               |        9 |         16 |       3 (=24/24) |        2 (=16/24) |
| 25               |       10 |         19 |       3 (=24/24) |        3 (=24/36) |
| 100              |       15 |         29 |       4 (=32/32) |        4 (=32/48) |

## Подготовка СУБД

Deckhouse Commander работает с СУБД PostgreSQL. Для корректной работы Deckhouse Commander необходимы
расширения [plpgsql](https://postgrespro.ru/docs/postgresql/14/plpgsql) и [pgcrypto](https://postgrespro.ru/docs/postgresql/14/pgcrypto).

### Вариант 1: выделенная СУБД

Это рекомендуемый способ использования Deckhouse Commander в производственных средах. Для
использования Deckhouse Commander необходимо подготовить параметры подключения к БД.

### Вариант 2: модуль operator-postgres

Это не рекомендуемый способ для использования в производственных средах. Однако использование
`operator-postgres` удобно для более быстрого знакомства с Deckhouse Commander или для сред, где нет
высоких требований к доступности и поддержке.

Модуль Deckhouse Commander важно включить после того, как в кластере появились CRD из модуля `operator-postgres`.

Модуль `operator-postgres` использует оператор PostgreSQL.
Вы можете использовать собственную инсталляцию postgres-operator версии не ниже `v1.10.0`.

#### Шаг 1: включение operator-postgres

Сначала нужно включить модуль оператора postgres и дождаться его включения

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-postgres
spec:
  enabled: true
```

#### Шаг 2: завершение установки

Чтобы удостовериться, что модуль включен, дождитесь, когда очередь задач Deckhouse станет пустой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
```

И проверьте наличие необходимых CRD в кластере:

```shell
kubectl get crd | grep postgresqls.acid.zalan.do
```

## Включение Deckhouse Commander

{{< alert level="info" >}}
Полный перечень параметров конфигурации приведен в разделе [Настройка](./configuration.html)
{{< /alert >}}

Чтобы включить Deckhouse Commander, создайте ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: commander
spec:
  enabled: true
  version: 1
  settings:
    postgres:
      mode: External
      external:
        host: "..."     # Обязательное поле
        port: "..."     # Обязательное поле
        user: "..."     # Обязательное поле
        password: "..." # Обязательное поле
        db: "..."       # Обязательное поле
```
