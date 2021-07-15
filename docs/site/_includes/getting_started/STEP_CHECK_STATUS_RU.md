Просмотр состояния Kubernetes-кластера возможен сразу после (или даже во время) установки Deckhouse Platform.
{%- if include.mode == "baremetal" or include.mode == "cloud" %} По умолчанию `.kube/config`, используемый для доступа к Kubernetes, генерируется на хосте с кластером. Если подключиться к этому хосту по SSH (из-под root), для взаимодействия с Kubernetes можно воспользоваться стандартными инструментами, такими как `kubectl`.
{%- endif %}

Например, посмотреть состояние кластера можно следующей командой:

```shell
kubectl -n d8-system get deployments/deckhouse
```

В ответе deployment с именем `deckhouse` должен иметь статус `READY 1/1` — это будет свидетельствовать о том, что установка модулей завершена, кластер готов для дальнейшего использования.

Для более удобного контроля за кластером доступен модуль с официальной веб-панелью для Kubernetes — [dashboard](/ru/documentation/v1/modules/500-dashboard/). Он активируется по умолчанию после установки и доступен по адресу `https://dashboard<значение параметра publicDomainTemplate>` с уровнем доступа `User`. (Подробнее про уровни доступа см. в документации по [модулю user-authz](/ru/documentation/v1/modules/140-user-authz/).)

Логи Deckhouse Platform ведутся в формате JSON. Для их просмотра «на лету» удобно использовать связку с [jq](https://stedolan.github.io/jq/download/). Вот несколько вариантов, которые вам могут пригодиться:
- Лаконичная команда просмотра событий в сокращенной форме:
  ```shell
  kubectl logs -n d8-system deployments/deckhouse -f --tail=10 | jq -rc .msg
  ```
- Просмотр событий "в цвете" с выводом информации о времени и названии модуля:
  ```shell
kubectl -n d8-system logs deploy/deckhouse -f | jq -r 'select(.module != null) | .color |= (if .level == "error" then 1 else 4 end) |
"\(.time) \u001B[1;3\(.color)m[\(.level)]\u001B[0m\u001B[1;35m[\(.module)]\u001B[0m - \u001B[1;33m\(.msg)\u001B[0m"'
```
- Просмотр событий конкретного модуля. Пример для модуля `node-manager`:
  ```shell
kubectl -n d8-system logs deploy/deckhouse -f | jq -r --arg mod node-manager 'select(.module == $mod) |
"\(.time) [\(.level)][\(.module)] - \(.binding) - \(.msg)"'
```

Для полноценного мониторинга состояния кластера существует специальный [набор модулей](/ru/documentation/v1/modules/300-prometheus/) на базе Prometheus.
