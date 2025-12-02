---
title: Что делать в случае проблем с добавлением узла в кластер через Cluster API Provider Static?
permalink: ru/faq-common/caps-adding-node-problems.html
lang: ru
---

Если при добавлении узла в кластер через Cluster API Provider Static (CAPS) он остается в статусе `Pending` или в `Bootstraping`, выполните следующие действия:

1. Проверьте корректность ключей доступа, указанных в ресурсе SSHCredentials. Убедитесь, что имя пользователя и SSH-ключ, указанные в SSHCredentials, верны.

1. На узле, с добавлением которого возникла проблема, проверьте наличие в `authorized_keys` публичного ключа, соответствующего приватному из SSHCredentials. Пример команды для проверки:

   ```shell
   cat ~/.ssh/authorized_keys
   ```

1. Проверьте количество узлов, указанное в NodeGroup, в которую должен входить добавляемый узел. Убедитесь, что не превышено максимальное количество узлов.

1. Проверьте статус службы bashible.service на узле, с добавлением которого возникла проблема:

   ```shell
   systemctl status bashible.service
   ```

   Он должен быть в статусе `active (running)`. Если сервис находится в статусе `inactive` или `failed` — служба не запустилась. Это указывает на проблему в процессе настройки.

1. Если указанные выше шаги не помогли решить проблему, удалите из кластера ресурсы StaticInstance и Node для проблемного узла, чтобы система попыталась создать их заново. Для этого:

   - Получите список узлов и найдите в нем проблемный:

     ```shell
     d8 k get nodes
     ```

   - Найдите соответствующий ресурс StaticInstance:

     ```shell
     kubectl get staticinstances -n <namespace-name>
     ```

   - Удалите проблемный узел:

     ```shell
     kubectl delete node <node-name>
     ```

   - Удалите соответствующий ресурс StaticInstance:

     ```shell
     kubectl delete staticinstances -n <namespace-name> <static-instance-name>
     ```
