---
title: Что делать в случае проблем с добавлением узла в кластер через Cluster API Provider Static?
permalink: ru/faq-common/caps-adding-node-problems.html
lang: ru
---

Если при добавлении узла в кластер через Cluster API Provider Static (CAPS) он остается в статусе `Pending` или в `Bootstrapping`, выполните следующие действия:

1. Проверьте корректность ключей доступа, указанных в ресурсе [SSHCredentials](/modules/node-manager/cr.html#sshcredentials). Убедитесь, что имя пользователя и SSH-ключ, указанные в [SSHCredentials](/modules/node-manager/cr.html#sshcredentials), верны.

1. На узле, с добавлением которого возникла проблема, проверьте наличие в `authorized_keys` публичного ключа, соответствующего приватному из [SSHCredentials](/modules/node-manager/cr.html#sshcredentials). Пример команды для проверки:

   ```shell
   cat ~/.ssh/authorized_keys
   ```

1. Проверьте количество узлов, указанное в [NodeGroup](/modules/node-manager/cr.html#nodegroup), в которую должен входить добавляемый узел. Убедитесь, что не превышено максимальное количество узлов.

1. Проверьте статус службы `bashible.service` на узле, с добавлением которого возникла проблема:

   ```shell
   systemctl status bashible.service
   ```

   Он должен быть в статусе `active (running)`. Если сервис находится в статусе `inactive` или `failed` — служба не запустилась. Это указывает на проблему в процессе настройки.

1. Если указанные выше шаги не помогли устранить проблему, удалите из кластера проблемный узел и его ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance), чтобы система попыталась создать их заново. Для этого:

   - Получите список узлов и найдите в нем проблемный:

     ```shell
     d8 k get nodes
     ```

   - Найдите соответствующий ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance):

     ```shell
     d8 k get staticinstances -n <namespace-name>
     ```

   - Удалите проблемный узел:

     ```shell
     d8 k delete node <node-name>
     ```

   - Удалите соответствующий ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance):

     ```shell
     d8 k delete staticinstances -n <namespace-name> <static-instance-name>
     ```
