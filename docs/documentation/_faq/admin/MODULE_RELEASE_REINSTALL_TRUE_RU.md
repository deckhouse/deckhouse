---
title: Что делать, если образ модуля не скачался и модуль не переустановился?
lang: ru
---

В некоторых случаях может возникнуть проблема с автоматическим скачиванием образа и переустановкой модуля. Среди этих случаев:

- Повреждение файловой системы или другие проблемы, из-за которых образ модуля стал невалидным.
- Переключение на использование другого хранилища образов.
- Переключение с одной редакции DKP на другую.

При этом модуль может находиться в статусе `Ready`. А ошибка возникает в подах модуля. Чтобы найти проблемный под используйте команду:

```shell
d8 k -n d8-<module-name> get pods
```

У проблемного пода будет статус, отличающийся от `Running`.

Для просмотра информации о поде используйте команду:

```shell
d8 k -n d8-<module-name> describe pod <pod-name>
```

Пример сообщения об ошибке в поде при проблеме со скачиванием образа и переустановкой модуля:

```console
Failed to pull image "registry.deckhouse.ru/deckhouse/ce/modules/console@sha256:a12b4f8de1d997005155d0ba0a7c968a015dd8d18bb5d54645ddb040ddab1ef4": rpc error: code = NotFound desc = failed to pull and unpack image "registry.deckhouse.ru/deckhouse/ce/modules/console@sha256:a12b4f8de1d997005155d0ba0a7c968a015dd8d18bb5d54645ddb040ddab1ef4": failed to resolve reference ...
```

Чтобы скачать образ и переустановить модуль, с которым возникла проблема:

1. Получите список релизов модуля:

   ```shell
   d8 k get mr -l module=my-module
   ```

   Пример вывода:

   ```console
   NAME               PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   my-module-v3.7.4   Superseded                   5d23h
   my-module-v3.7.5   Deployed                     5d23h
   ```

   Найдите в списке релиз модуля, развернутый в кластере (он должен иметь статус `Deployed`).

1. Добавьте к развернутому релизу аннотацию `modules.deckhouse.io/reinstall=true`:

   ```shell
   d8 k annotate mr my-module-v3.7.5 modules.deckhouse.io/reinstall=true
   ```

После добавления аннотации образ модуля заново скачивается из хранилища образов, модуль валидируется с текущими настройками из `ModuleConfig` и устанавливается в кластер. После успешной переустановки аннотация автоматически удаляется из `ModuleRelease`.

Чтобы убедиться, что переустановка модуля прошла успешно, и все поды работают, используйте команду:

```shell
d8 k -n d8-<module-name> get pods
```

Все поды модуля должны иметь статус `Running`. Пример:

```console
NAME                                READY   STATUS    RESTARTS   AGE
backend-567d6c6cdc-g5qgt            1/1     Running   0          2d2h
frontend-7c8b567759-h8jdf           1/1     Running   0          2d2h
observability-gw-86cf75f5d6-7xljh   1/1     Running   0          2d2h
```
