# Разработка Antiopa

## Переключение уже установленной antiopa на другую версию

```
kubectl -n antiopa test edit deploy/antiopa
```

Меняем версию образа `registry.flant.com/sys/antiopa:stable`, например, на `registry.flant.com/sys/antiopa:master`.

## Как проверить мою версию

CI настроен так, что каждый бранч всегда собирается в образ и доступен по адресу `registry.flant.com/sys/antiopa/dev:<BRANCH>`. Все что нужно, чтобы проверить версию из еще не принятого бранча — изменить образ в deployment'е antiopa.


## Процесс релиза

Когда в master набралось достаточно изменений, чтобы сделать релиз, делается следующее:
1. Проверяем изменения, выкатив master
    - В последнем pipeline master'а нажимается кнопка `master` (в колонке `deploy`), при этом dapp делает push образа, который уже есть в `antiopa/dev:master`, в `antiopa:master` и все инсталляции antiopa, подключенные к версии master, обновляются.
    - Проверяем логи на достаточном наборе кластеров подключенных к версии `master`, чтобы быть уверенным, что все хорошо. Если есть проблемы — заводим issue, исправляем в MR, принимаем эти MR и снова выкатываем версию `master`.
2. Создаем релиз
    - Определяем `название релиза` в формате `YYYY-MM-DD.N` (где N, это номер релиза за день, начиная с 1).
    - Переименовываем Milestone'ы:
        - `current` переименовывать в `<название релиза>`,
        - создаем новый `current`, в который передвигаем все оставшиеся задачи и MR'ы,
        - если задач и MR'ов нет, то просто `next` переименовываем в `current`, `after-next` в `next` и создаем новый, пустой, `after-next`.
    - Составляем подробное описание изменений, предназначенное, в первую очередь, для DevOps команд (чтобы они могли четко понимат, какие изменения есть в релизе).
    - Ставим git tag `<название релиза>` на ветку master, в описание тега размещаем подробно описание релиза. CI настроен так, что при этом dapp сделает push образа `antiopa:<tag>`.
3. Выкатываем релиз сначала на `ea`, затем на `stable`
    - Уведомляем DevOps команды в соответсвтующем канале в slack.
    - Для выката в pipeline tag'а есть соответствующие кнопки `ea` и `stable`. При нажатии на эти кнопки dapp делает push образа, который уже есть в `antiopa:<tag>`, в `antiopa:ea` или `antiopa:stable` соответственно.
    - Как именно проверять корерктность выката на `ea` и сколько выжидать до выката на `stable` — зависит от изменений, которые попали в релиз.

## Соглашение об именовании

* Для всего, что написано на Shell — мы используем [Shell Style Guid](https://google.github.io/styleguide/shell.xml).
* Для идентификаторов в Kubernetes мы используем [соответствующий стандарт](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/identifiers.md).
* Для Helm Values мы используем camelCase как в Kubernetes, согласно [официальным рекомендациям](https://github.com/kubernetes/helm/blob/master/docs/chart_best_practices/values.md#naming-conventions). Исключение: проброс целиком части values в Kubernetes (например как в случае с nodeSelector).
* Для названий Helm Chart'ов мы используем маленькие буквы и дефисы, согласно [официальные рекомендии](https://github.com/kubernetes/helm/blob/master/docs/chart_best_practices/conventions.md#chart-names).
* Название модуля должно всегда соответствовать названию Helm Chart'а.
* Для названий образов модулей (тех, которые лежат в `modules/*/images/*`) мы используем маленькие буквы и дефисы (чтобы ссылка на image, которая так же содержит имя модуля, не была в разных стилях).
* Переменные в Go-шаблонах в Helm Chat'ах мы именуем camelCase'ом, как это принято в Go.
* На структуру Helm Chart'ов у нас пока нет никаких соглашений, так что каждый как захочет!
* Если одно и тоже название нам нужно использовать в разных местах, например в ConfigMap (идентификатор Kubernetes), Helm Values, Shell и Go — мы используем в каждому случае свое соглашение об именовании, соответственно: `use-proxy-protocol`, `useProxyProtocol`, `use_proxy_protocol`. Согласно этому правилу название модуля и название имени образа модуля (которые содержат дефис), когда они используются в Helm Values, становятся camelCase.
* Если на то нет других причин, в названиях фалов, в качестве разделителей, мы старемся использовать нижние подчеркивания и точки, а не дефисы.
* При именовании объектов в kubernetes мы придерживаемся следующей статегии:
    * namespace называем так-же как модуль, но с приставкой `kube-` (символизируя этим, что это НЕ пользовательское приложение, а "системный компонент"), например, модуль `prometheus`, а namespace — `kube-prometheus`.
    * все глобальные объекты и объекты, которые мы размещаем за пределами наших namespace'ов, называем так-же как namespace или используем название namespace'а в качестве префикса, например: `clusterrole/kube-prometheus:node-exporter`, `service/kube-prometheus-discovery-of-kube-controller-manager` (в namespace `kube-system`), `daemonset/kube-prometheus-kube-control-plane-proxy` (в namespace `kube-system`).

## Значения в Helm Values

* Для bool значений используем всегда настоящий bool, а не строку. И используем слова true или false, а не любые другие.
* Для констант используем соглашение, как в Kubernetes — с большой буквы, CamelCase. Например: `LoadBalancer`, `ClusterIP`.

## Обязательные лейблы antiopa

У всех ресурсов, которые **создаются и управляются antiopa**, должны стоять два лейбла:
* `heritage: antiopa`
* `module: <имя модуля>`

**Внимание!!!* Это не означает, что эти лейблы нужно ставить на pod'ы, создаваемые другими контроллерами и пр. Нет. Указанные лейблы необходимо ставить только на первичные ресурсы, находящиеся под управлением antiopa.

## Рекомендации по использованию лейблов

Рекомендуется использовать лейблы `app` и `component`.

## Содержимое Chart.yaml

Там должно быть только название и версия `0.0.1`.

## Values для модулей

Values для конкретного модуля объявляются в глобальном ключе с именем модуля (сконвертированным в camelCase).

```
myModule:
  mysql:
    rootPassword: password
    database: main
    user:
      name: user
      password: password
```

Внутри ConfigMap antiopa'ы данный пример будет выглядеть так:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: antiopa
data:
  my-module-values: |
    myModule:
      mysql:
        rootPassword: password
        database: main
        user:
          name: user
          password: password
...
```

