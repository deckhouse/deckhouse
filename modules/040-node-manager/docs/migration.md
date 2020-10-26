---
title: "Управление узлами: FAQ"
search: миграция flant.com, dedicated.flant.com, node-role.flant.com, D8DeprecatedNodeSelectorOrTolerationFound, D8DeprecatedNodeGroupLabelOrTaintFound, D8DeprecatedNodeSelectorOrTolerationFoundInCluster, D8DeprecatedNodeGroupLabelOrTaintFoundInCluster
---

##### ✊ Миграция `flant.com` ➡️ `deckhouse.io`

> `<имя модуля>` – множество значений из имен директорий в папке `/modules` данного репозитория, без цифрового префикса.

###### Соглашения по миграции:
1. В ранее использованных `labels` `node-role.flant.com/(system|frontend|monitoring|<имя модуля>)` домен `flant.com` изменяется на `deckhouse.io`.
1. В ранее использованных `taints`, с ключами `dedicated.flant.com` и значениями `(system|frontend|monitoring|<имя модуля>)` домен `flant.com` изменяется на `deckhouse.io`.
1. Остальные `labels` вида `node-role.flant.com/production` или `node-role.flant.com/whatever` могут быть использованы далее без изменений. 
1. Остальные `taints`, с ключами `dedicated.flant.com` и значениями `production` или `whatever` могут быть использованы далее без изменений.

###### Последовательность для беспростойной миграции
1. По ресурсами из алертов `D8DeprecatedNodeSelectorOrTolerationFound` произвести следующие операции:
   - В ключах `nodeSelector` / `nodeAffinity`, попадающих под выражение `node-role.flant.com/(system|frontend|monitoring|<имя модуля>)` – сменить доменное имя на новое `node-role.deckhouse.io`.
   - Для `tolerations` с ключами `dedicated.flant.com` и значениями `(system|frontend|monitoring|<имя модуля>)` добавить еще один – с ключом `dedicated.deckhouse.io` и таким же значением.
1. Если есть такой же алерт `D8DeprecatedNodeSelectorOrTolerationFound` для `ConfigMap` `deckhouse`, то это означает, что требуется внести правки в конфигурации модулей, описанных в `cm/deckhouse`. Список операций, для каждого модуля, такой же, что и в предыдущем пункте.
   > Имейте в виду, что пока алерты не погаснут, так как продолжат срабатывать на `dedicated.flant.com` в `tolerations`. Так и задумано, идём дальше.
1. Для `NodeGroup` из алерта `D8DeprecatedNodeGroupLabelOrTaintFound` сделать следующее:
   - Изменить `nodeTemplate.labels` в соответствии с описанными выше соглашениями.
   - Изменить `nodeTemplate.taints` в соответствии с описанными выше соглашениями.
1. Удалить, прежде оставленные, `tolerations` `dedicated.flant.com`:
   - В ресурсах из алертов `D8DeprecatedNodeSelectorOrTolerationFound`.
   - В `ConfigMap` `deckhouse`.
   > После этого алерты на ресурсы смогут погаснуть. 

> ☝️ Если простой приложений не страшен сразу меняйте `tolerations` на новые. Это позволит избежать второй итерации из последнего пункта.  
