---
title: Что делать при проблемах с применением настроек DexProvider?
subsystems:
  - iam
lang: ru
---

Если вы изменили настройки DexProvider в модуле `user-authn` и при этом наблюдается одна из следующих проблем:

- не видно никаких изменений (настройки не применяются);
- при попытке входа в веб-интерфейс платформы с любым типом авторизации возникает ошибка `500 Internal Server Error` без подробного описания.

Выполните следующие действия:

1. Проверьте статус подов деплоймента dex:

   ```shell
   d8 k -n d8-user-authn get pod
   ```

   Пример вывода:

   ```shell
   NAME                                    READY   STATUS    RESTARTS   AGE
   dex-5ddb779b7d-6pbhs                    2/2     Running   0          20h
   kubeconfig-generator-7c46977b9f-5kdmc   1/1     Running   0          20h
   ```

   Если модуль работает нормально и в [DexProvider](/modules/user-authn/cr.html#dexprovider) указана корректная конфигурация, все поды будут в статусе `Running`.

1. Посмотрите логи проблемного пода:

   ```shell
   d8 k -n d8-user-authn logs dex-<pod-name>
   ```

   На основе информации из логов исправьте конфигурацию в ресурсе [DexProvider](/modules/user-authn/cr.html#dexprovider) и дождитесь перезапуска подов dex. В течение нескольких минут поды перезапустятся автоматически, а веб-интерфейс платформы (находится по адресу `console.<ШАБЛОН_ИМЕН_КЛАСТЕРА>`) станет доступен и в нем отразятся внесенные изменения в ресурс [DexProvider](/modules/user-authn/cr.html#dexprovider).
