Deckhouse
=======

Запуск в minikube
-----------------

```
minikube start
dapp kube minikube setup
# дать больше прав для kube-system, иначе не создастся clusterole для deckhouse (возможно починено в свежих версиях minikube)
kubectl create clusterrolebinding fixRBAC --clusterrole=cluster-admin --serviceaccount=kube-system:default

dapp dimg build
dapp dimg push :minikube
dapp kube deploy --set global.env=minikube :minikube
```

Эти команды обёрнуты в скрипты:
```
development/scripts/setup_minikube_and_helm.sh
development/scripts/deploy_to_minikube.sh
```


Установка libgit2-24 на ubuntu 14.04
------------------------------------

Добавить /etc/apt/sources.list.d/xenial.conf

```
deb http://ru.archive.ubuntu.com/ubuntu/ xenial main restricted universe
deb http://security.ubuntu.com/ubuntu/ xenial-security main restricted universe
```

Добавить /etc/apt/preferences.d/xenial.pref

```
Package: *
Pin: release n=xenial
Pin-Priority: -10

Package: libgit2-24
Pin: release n=xenial
Pin-Priority: 500

Package: libgit2-dev
Pin: release n=xenial
Pin-Priority: 500
```

Обновить пакеты, поставить библиотеку
```
sudo apt-get update

sudo apt-get install libgit2-24 libgit2-dev

go get -v -d gopkg.in/libgit2/git2go.v24

```

Всё, после этого должен работать `go run play-git2go/main.go`



Эксперименты с Kubernetes API
=============================

После деплоя в kubernetes можно зайти в pod и экспериментировать с kubernetes API:

```
root@deckhouse-729075578-lhnj1:/# . kube_api
bash: Bearer: command not found
https://192.168.0.1:443*
  Usage:
$ GET /api/v1/nodes - list all nodes in cluster
$ GET /api/v1/namespace/$KUBE_NS/pods - list pods in current namespace
$ GET /apis/extensions/v1beta1/ingresses - list all inggreses in cluster

$ GET /api/v1/nodes | jq '.["items"][] | { name: .metadata.name, labels: .metadata.labels }'
  - list all nodes with name and labels

PATCH /api/v1/namespaces/$KUBE_NS/pods/deckhouse-1344919674-zjfkm '[{"op":"add","path":"/metadata/labels/qwe", "value": "qwe" }]' -H "Content-Type:application/json-patch+json"
  - add a new label to pod
  https://stackoverflow.com/a/36163917

```

```
root@deckhouse-729075578-lhnj1:/# GET /api/v1/namespaces/$KUBE_NS/pods
{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/namespaces/deckhouse-stage/pods",
    "resourceVersion": "17120231"
  },
  "items": [
    {
      "metadata": {
        "name": "deckhouse-729075578-lhnj1",
        "generateName": "deckhouse-729075578-",
        "namespace": "deckhouse-stage",
        "selfLink": "/api/v1/namespaces/deckhouse-stage/pods/deckhouse-729075578-lhnj1",
        "uid": "0137da54-a528-11e7-9d01-901b0ebb25f4",
        "resourceVersion": "17119550",
        "creationTimestamp": "2017-09-29T15:08:09Z",
        "labels": {
          "pod-template-hash": "729075578",
...
```
