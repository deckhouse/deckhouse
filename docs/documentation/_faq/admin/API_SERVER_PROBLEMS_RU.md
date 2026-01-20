---
title: Что делать при высокой нагрузке на API-сервер?
subsystems:
- kubernetes_scheduling
lang: ru
---

О проблемах с нагрузкой и потреблением памяти API-сервером могут говорить следующие признаки:

- `kubectl (d8)` долго отвечает или не отвечает совсем (команды выполняются медленно или не выполняются);
- в кластере происходит пересоздание подов без явных причин.

При наличии этих признаков выполните следующие действия:

1. Проверьте потребление ресурсов подами API-сервера. Для этого используйте команду:

   ```shell
   d8 k -n kube-system top po -l component=kube-apiserver
   ```

   Обратите внимание на потребление памяти (`MEMORY`) и `CPU`.

   Пример вывода:

   ```console
   NAME                               CPU(cores)   MEMORY(bytes)
   kube-apiserver-sandbox1-master-0   251m         1476Mi
   ```

1. Проверьте метрики в Grafana.

   Для просмотра метрик откройте дашборд «Home» → «Dashboards» → «Kubernetes Cluster» → «Control Plane Status». Изучите графики, связанные с API-сервером («Kube-apiserver CPU Usage», «Kube-apiserver Memory Usage», «Kube-apiserver latency» и т.д.).

1. Изучите [аудит-логи API-сервера](/modules/control-plane-manager/#аудит), чтобы выявить источник высокого потребления памяти. Одна из частых причин высокого потребления памяти — большое количество запросов.
