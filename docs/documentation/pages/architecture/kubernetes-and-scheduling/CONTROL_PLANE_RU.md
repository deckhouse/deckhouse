---
title: Control plane Kubernetes-кластера
permalink: ru/architecture/kubernetes-and-scheduling/control-plane/
lang: ru
search: control plane
---

В платформе Deckhouse Kubernetes Platform используется *ванильный* Kubernetes кластер. Control plane Kubernetes-кластера включает в себя следущие стандартные компоненты:

1. **kube-apiserver** - сервер API Kubernetes, обслуживает операции REST API, предоставляет интерфейс для общего состояния кластера, через который взаимодействуют все остальные компоненты кластера, валидирует ресурсы Kubernetes API и сохраняет их в хранилище **etcd**. Включает следующие контейнеры:

   * **kube-apiserver** - основной контейнер.  
   * **kube-apiserver-healthcheck** - sidecar-контейнер, который позволяет проверять работоспособность **kube-apiserver**, не включая анонимную аутентификацию и не включая порт, не прошедший проверку подлинности. Использует сертификат клиента для аутентификации на api-сервере. [Open-source разработка](https://github.com/kubernetes/kops/blob/master/cmd/kube-apiserver-healthcheck).

2. **etcd** - распределённое хранилище ключ-значение, где хранятся вся конфигурация и ресурсы Kubernetes-кластера.
3. **kube-scheduler** - планировщик Kubernetes, анализирует ресурсы нод и размещает поды оптимально, учитывая affinity и taints.
4. **kube-controller-manager** - диспетчер контроллеров Kubernetes, запускает циклы контроллеров,  которые мониторят и корректируют состояние стандартных ресурсов Kubernetes, пытаясь приблизить их текущее состояние к желаемому. Примерами контроллеров, которые поставляются с Kubernetes, являются *replication controller*, *endpoints controller*, *namespace controller* и *serviceaccounts controller*.

Взаимодействие компонентов Kubernetes control plane изображено [на схеме архитектуры модуля control-plane-manager](control-plane-management/).
