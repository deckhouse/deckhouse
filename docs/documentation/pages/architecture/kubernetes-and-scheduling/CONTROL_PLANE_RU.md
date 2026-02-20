---
title: Control plane кластера
permalink: ru/architecture/kubernetes-and-scheduling/control-plane/
lang: ru
search: архитектура control plane
description: Архитектура control plane кластера в Deckhouse Kubernetes Platform.
---

В Deckhouse Kubernetes Platform (DKP) используется стандартный («vanilla») кластер Kubernetes. Control plane кластера включает в себя следующие базовые компоненты:

1. **kube-apiserver** — API-сервер Kubernetes. Обрабатывает REST-запросы, предоставляет интерфейс доступа к общему состоянию кластера, через который взаимодействуют все остальные компоненты, валидирует ресурсы Kubernetes API и сохраняет их в хранилище **etcd**. Включает следующие контейнеры:

   * **kube-apiserver** — основной контейнер;
   * **kube-apiserver-healthcheck** - сайдкар-контейнер, который позволяет проверять работоспособность **kube-apiserver** без включения анонимной аутентификации и без открытия порта, не прошедшего проверку подлинности. Использует клиентский сертификат для аутентификации на API-сервере. Является [Open Source-продуктом](https://github.com/kubernetes/kops/blob/master/cmd/kube-apiserver-healthcheck).

2. **etcd** — распределённое хранилище типа «ключ-значение», где хранится вся конфигурация и ресурсы Kubernetes-кластера.

3. **kube-scheduler** — планировщик Kubernetes. Анализирует ресурсы узлов и размещает поды с учетом ограничений и правил, таких как affinity и taints.

4. **kube-controller-manager** — диспетчер контроллеров Kubernetes. Запускает циклы контроллеров, которые отслеживают и корректируют состояние стандартных ресурсов Kubernetes, приводя их к желаемому состоянию. Примеры контроллеров, которые поставляются с Kubernetes: replication controller, endpoints controller, namespace controller и ServiceAccount controller.

Взаимодействие компонентов control plane Kubernetes изображено на [схеме архитектуры модуля `control-plane-manager`](../control-plane-management/).
