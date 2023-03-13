---
title: "Модуль dashboard"
webIfaces:
- name: dashboard
---

Устанавливает [Web UI](https://github.com/kubernetes/dashboard) Kubernetes Dashboard для ручного управления кластером, который интегрирован с модулями [user-authn](../../modules/150-user-authn/) и [user-authz](../../modules/140-user-authz/) (доступ в кластер осуществляется от имени пользователя и с учетом его прав).

Если модуль работает по HTTP, он использует минимальные права с ролью `User` из модуля: [user-authz](../../modules/140-user-authz/).

> **Важно!** Для работы модуля dashboard необходим включенный модуль `user-authz`.

Kubernetes Dashboard среди прочего позволяет:
- управлять Pod'ами и другими высокоуровневыми ресурсами;
- получать доступ в контейнеры через web-консоль для отладки;
- просматривать логи отдельных контейнеров.
