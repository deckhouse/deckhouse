---
title: Что делать если образ модуля не скачался и модуль не переустановился?
lang: ru
---

В некоторых случаях может возникнуть проблема с автоматическим скачиванием образа и переустановкой модуля. Среди этих случаев:

- Повреждение файловой системы или другие проблемы, из-за которых образ модуля стал невалидным.
- Переключение на использование другого хранилища образов.
- Переключение с одной редакции DKP на другую.

Пример сообщения об ошибке при проблеме со скачиванием образа и переустановкой модуля:

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

Чтобы убедиться, что образ скачался и переустановка модуля прошла успешно, используйте команду:

```shell
d8 k get modules
```

Модуль должен находиться в статусе `Ready` (колонка `PHASE` в выводе команды).
