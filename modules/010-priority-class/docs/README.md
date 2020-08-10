---
title: "Модуль priority-class"
---

## Назначение

Модуль создает в кластере набор [priority class'ов](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) (и проставляет их компонентам установленным Deckhouse) что дает возможность применять в проектах согласованные имена priority class'ов и обеспечить работу приоритезации при шедулинге.

##  Конфигурация

По умолчанию — **включен** в кластерах начиная с версии 1.11.

В спецификации пода необходимо установить [соответствующий](#как-работает) [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority).
Очень важно правильно выставлять `priorityClassName`. Если есть сомнения - спросите коллег.

> Любой установленный `priorityClassName` не уменьшит приоритета пода, т.к. если `priority-class` у пода не установлен, шедулер считает его самым низким — `develop`.

##  Как работает

Модуль устанавливает 10 priority class'ов (написаны в порядке приоритета от большего к меньшему) и использует один системный (system-cluster-critical):

| Priority Class          | Описание                                                                                                                                                            | Значение   |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| `system-cluster-critical` | Компоненты кластера, без которых его корректная работа полностью невозможна.<br>`kube-dns`, `coredns`, `kube-proxy`, `flannel`, `kube-api-server`, `kube-controller-manager`, `kube-scheduler`, `cluster-autoscaler`, `dns-controller`.                             | 2000000000 |
| `cluster-critical`        | Тоже самое, что и `system-cluster-critical`, но для компонентов которые устанавливаются в отличные от `kube-system` namespace'ы | 1000000000 |
| `production-high`         | Stateful приложения, в production окружении отсутствие которых приводит к полной недоступности сервиса или потере данных (postgresql, memcached, redis, mongo, ...). | 9000       |
| `cluster-high`            | Ключевые компоненты кластера, выход из строя которых влияет на работу всего кластера и приложений.<br>`nginx-ingress`.                                              | 8000       |
| cluster-medium          | Компоненты кластера, влияющие на мониторинг (алерты, диагностика) кластера и автоскейлинг. Без мониторинга мы не можем оценить масштабы происшествия, без автоскейлинга мы не сможем дать приложениям необходимые ресурсы.<br>`deckhouse`, `prometheus`, `kube-state-metrics`, `madison-proxy`, `node-exporter`, `pormetheus-proxy`, `trickster`, `prometheus-metrics-adapter`, `extended-monitoring`, `grafana`                       | 7000       |
| `production-medium`       | Основные stateless приложения в production окружении, которые отвечают за работу сервиса для посетителей.                                                            | 6000       |
| `deployment-machinery`    | Компоненты кластера с помощью которых происходит деплой/билд в кластер (helm, werf).<br>`kube-system/tiller-deploy`                                                                                 | 5000       |
| `production-low`          | Приложения в production окружении (кроны, админки, batch-процессинг, ...), без которых можно прожить некоторое время. Если batch или крон никак нельзя прерывать, то он должен быть в production-medium, а не здесь.                                          | 4000       |
| `staging`                 | Staging окружения для приложений.                                                                                                                                    | 3000       |
| `cluster-low`             | Компоненты кластера, без которых возможна эксплуатация кластера, но которые желательны. <br>`prometheus-operator`, `node-ping`, `okmeter`, `dashboard`, `dashboard-oauth2-proxy`, `cert-manager`, `prometheus-longterm`                                                                              | 2000       |
| `develop` (default)       | Develop окружения для приложений. Класс по умолчанию, если не проставлены иные классы.                                                                               | 1000       |

### Что такое priority class

[Priority Class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption) — это функционал scheduler'а, который позволяет учитывать приоритет пода (из его принадлежности к классу) при шедулинге.

К примеру, при выкате в кластер подов с `priorityClassName: production-low`, если в кластере не будет доступных ресурсов для данного пода, то kubernetes начнет evict'ить поды с наименьшим приоритетом в кластере.
Т.е. сначала будут выгнаны все поды с `priorityClassName: develop`, потом с `cluster-low` и так далее.

При выставлении priority class очень важно понимать к какому типу относится приложение и в каком окружении оно будет работать.
