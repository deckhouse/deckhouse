---
title: "Kubernetes Web UI (Dashboard)"
---

С Deckhouse поставляется [Kubernetes Dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/) – Web UI для ручного управления кластером.

Данный интерфейс интегрирован с модулями [user-authn](/modules/150-user-authn/) и [user-authz](/modules/140-user-authz/), поэтому доступ в кластер осуществляет от имени пользователя, с учетом его прав.

Kubernetes Dashboard среди прочего позволяет:
- управлять pod’ами и другими высокоуровневыми ресурсами
- получать доступ в контейнеры через веб-консоль для отладки
- просматривать логи отдельных контейнеров
