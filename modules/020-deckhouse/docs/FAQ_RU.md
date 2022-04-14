---
title: "Модуль deckhouse: FAQ"
---

## Как запустить kube-bench в кластере?

Вначале необходимо зайти внутрь Pod'а Deckhouse:
```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- bash
```

Далее, необходимо выбрать, на каком узле запустить kube-bench.

* Запуск на случайном узле:
  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
  ```

* Запуск на конкретном узле, например на control-plane:
  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | yq r - -j | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
  ```

Далее можно проверить результат выполнения
```shell
kubectl logs job.batch/kube-bench
```
