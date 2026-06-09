## Общая инфраструктура
Корневой Taskfile.yaml подключает четыре набора тестов как отдельные Task-инклюды. Общая конфигурация Chainsaw в chainsaw-config.yaml: 
длинные таймауты на assert (15m), failFast: true, параллельность 1, поиск тестов в chainsaw-test.yaml.

Каждый сценарий в своей папке tests/<name>/ запускается через Task (run, run:quiet, dry-run, run:debug) и вызывает chainsaw test с JUnit-отчётом в ./reports/.

Общая нагрузка — tests/common/manifests/deployment.yaml: три реплики pause, nodeSelector: app=e2e-autoscaler-test, жёсткий pod anti-affinity по ноде, 
tolerations под taints dedicated=worker-100 и dedicated=worker-50 — чтобы поды могли сесть только на тестовые группы с соответствующими taints.

## 1. ca-scale-from-zero-dvp
   Платформа: кластер DVP / Cluster API.

Цель: проверить scale-from-zero и Priority Expander: при двух NodeGroup с разными приоритетами CA выбирает группу с большим приоритетом.

Ключевые шаги:

Убедиться, что deployment cluster-autoscaler в d8-cloud-instance-manager готов.
В аргументах контейнера CA есть провайдер clusterapi.
Есть эталонный DVPInstanceClass worker.
Очистка хвостов (e2e-worker-100/50, e2e-worker-small).
Клон worker → e2e-worker-small с уменьшенными ресурсами (jq).
kubectl rollout restart CA, пауза 15s.
Применение NodeGroup e2e-worker-100 (приоритет 100) и e2e-worker-50 (приоритет 50), обе с minPerZone: 0.
Деплой e2e-nginx.
По логам CA ждётся строка вида e2e-worker-100.*chosen as the highest available; при ошибке масштабирования — быстрый fail по failed to increase node group size.*e2e-worker.
Проверка: 3 ready пода; все на нодах с node.deckhouse.io/group=e2e-worker-100; нет нод у группы e2e-worker-50 (error-assert).
Отличие от Yandex: порядок «создать instance class → restart CA» и чтение логов только из контейнера cluster-autoscaler (без --all-containers).

## 2. ca-scale-from-zero-yandex
Платформа: Yandex Cloud.

Цель: то же, что у DVP: scale-from-zero + приоритет, побеждает высокоприоритетная группа e2e-worker-100.

Отличия от DVP:

Проверка аргументов CA на mcm (Machine Controller Manager), не clusterapi.
YandexInstanceClass: клон worker → e2e-worker-small (cores/memory/disk).
Restart CA до создания e2e-worker-small (другой порядок шагов).
Логи CA: --all-containers --since=10m.
Нет раннего assert на DVPInstanceClass; вместо этого assert на YandexInstanceClass worker.
Итоговые проверки те же: логи про выбор e2e-worker-100, поды только на этой группе, нод группы e2e-worker-50 нет.

## 3. ca-priority-fallback-dvp
   Платформа: DVP / CAPI.

Цель: не «успешный высокий приоритет», а fallback Priority Expander: высокоприоритетная группа сломана, после backoff CA должен перейти на рабочую низкоприоритетную.

Механика поломки:

e2e-worker-small — как в scale-from-zero.
Дополнительно e2e-worker-broken: клон worker с несуществующим virtualMachineClassName (DOES-NOT-EXIST).
e2e-worker-100 ссылается на broken IC (nodegroup-100-broken.yaml), e2e-worker-50 — на рабочий e2e-worker-small.
Сценарий проверок:

Сначала в логах ожидается, что приоритетно «выбран»/рассматривается e2e-worker-100 (chosen as the highest available — CA всё равно идёт по приоритету).
Долгий этап (до 30 минут): в логах одновременно e2e-worker-100.*not ready for scaleup и e2e-worker-50.*chosen as the highest available.
Три ready пода; все поды на нодах e2e-worker-50, не на 100.

## 4. ca-priority-fallback-yandex
   Платформа: Yandex Cloud.

Цель: тот же fallback, что и у DVP.

Механика поломки:

e2e-worker-broken: клон с невалидным imageID (fd8INVALID000000000).
Дальше тот же каркас: mcm, restart CA, nodegroup-100-broken + nodegroup-50, деплой, сначала логи про выбор 100, затем ожидание backoff 100 и выбора 50.

Отличия от DVP по таймингам: ожидание fallback до 60 минут (цикл длиннее), логи с --since=90m — под более медленные ошибки провайдера Yandex.

| Сценарий                      | Провайдер CA | Суть проверки                                | Успешный итог по нодам                   |
| ----------------------------- | ------------ | -------------------------------------------- |------------------------------------------|
| `ca-scale-from-zero-dvp`      | clusterapi   | Приоритет при scale-from-zero                | Все поды на `e2e-worker-100`, 50 без нод |
| `ca-scale-from-zero-yandex`   | mcm          | То же для Yandex                             | То же                                    |
| `ca-priority-fallback-dvp`    | clusterapi   | Fallback после нерабочей top-пriority группы | Все поды на `e2e-worker-50`              |
| `ca-priority-fallback-yandex` | mcm          | То же с битым image                          | Все поды на `e2e-worker-50`              |

Во всех сценариях cleanup удаляет тестовые NodeGroup, instance class и Deployment; 
при удалении деплоя дополнительно опрашиваются ноды с лейблом app=e2e-autoscaler-test до исчезновения (с предупреждением по таймауту, не fail теста).



## Что нужно на машине

| Инструмент    | Зачем                                          |
|---------------|------------------------------------------------|
| kubectl       | Доступ к кластеру, контекст выбран заранее     |
| Chainsaw      | `chainsaw test`, `chainsaw lint`               |
| Task (`task`) | Обёртки в `Taskfile.yml`                       |
| jq            | Скрипты клонирования `*InstanceClass` в тестах |

Установка Chainsaw (пример):

```
go install github.com/kyverno/chainsaw@latest
```

```
или бинарь с https://github.com/kyverno/chainsaw/releases
```

Проверка:
```
chainsaw version

task --version

kubectl cluster-info
```


## Требования к кластеру

### Общее

1. Deckhouse с модулем node-manager и облачными нодами (`CloudEphemeral`).
2. Cluster Autoscaler уже развёрнут и в статусе Ready:
  - Deployment `cluster-autoscaler` в namespace `d8-cloud-instance-manager`.
  - CA включается, если есть хотя бы одна `NodeGroup` с `nodeType: CloudEphemeral` и `minPerZone < maxPerZone` (см.`cluster_autoscaler_enabled` в `templates/_helpers.yaml`).
3. Priority Expander — в CA задано `--expander=priority,least-waste` (так в шаблоне модуля). 
4. Приоритеты групп попадают в ConfigMap `cluster-autoscaler-priority-expander` через hook `set_ng_priorities` по полю `spec.cloudInstances.priority` `NodeGroup`.
5. Права в кластере: создавать/удалять `NodeGroup`, `DVPInstanceClass`/`YandexInstanceClass`,`Deployment`; делать `rollout restart` CA; читать логи и события.

### Для DVP-сценариев (`*-dvp`)

- Провайдер DVP (или другой с `cloud-provider=clusterapi` в args CA).
- Существует `DVPInstanceClass` с именем `worker` (эталон для клонирования).
- В args deployment CA есть подстрока `clusterapi`.

### Для Yandex-сценариев (`*-yandex`)

- Облако Yandex Cloud, MCM.
- Существует `YandexInstanceClass` с именем `worker`.
- В args CA есть `mcm`.

### Ресурсы и риски

- Тесты создают группы `e2e-worker-100`, `e2e-worker-50`, уменьшенные instance class, деплой `e2e-nginx` (3 реплики с anti-affinity → до 3 новых ВМ).
- Делают restart `cluster-autoscaler`.
- Стоимость облака и время: scale-from-zero обычно десятки минут; fallback — до 30–60 мин (Yandex дольше).
- После теста cleanup удаляет ресурсы; ноды с лейблом `app=e2e-autoscaler-test` ждутся до ~10 мин (при таймауте — warning, не обязательно fail).

## Запуск

### 1. Lint без кластера (проверка YAML)

Из каталога нужного сценария:
```
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-dvp
task dry-run
```

### 2. Полный прогон

Вариант A: из каталога сценария:

```
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-yandex

task run # полный вывод + JUnit в ./reports/

task run:quiet # только ошибки и итог

task run:debug # pause-on-failure + fail-fast

```


Вариант B: из корня e2e через includes:

```
cd modules/040-node-manager/e2e/cluster-autoscaler

task ca-scale-from-zero-dvp:run

task ca-scale-from-zero-yandex:run

task ca-priority-fallback-dvp:run

task ca-priority-fallback-yandex:run

```

### 3. Напрямую через chainsaw

```
cd modules/040-node-manager/e2e/cluster-autoscaler/tests/ca-scale-from-zero-dvp

mkdir -p reports

chainsaw test --test-dir . \

--config ../../chainsaw-config.yaml \

--parallel 1 \

--report-format JUNIT-TEST \

--report-path ./reports/

```

### Контекст kubectl

Chainsaw использует текущий контекст `kubectl`. Перед запуском:

```
kubectl config current-context
kubectl get deployment cluster-autoscaler -n d8-cloud-instance-manager
kubectl get dvpinstanceclass worker   # для DVP
# или
kubectl get yandexinstanceclass worker   # для Yandex
```

Переменная `$NAMESPACE` в скриптах — namespace, который Chainsaw создаёт для теста (деплой `e2e-nginx` попадает туда).

## Таймауты (из `chainsaw-config.yaml`)

- assert: 15m
- apply: 60s
- delete/cleanup: 5–10m
- Отдельные шаги: опрос логов CA до 5–30 мин; fallback Yandex — до 60 мин

Планируйте 30–90 мин на один полный прогон в зависимости от сценария и облака.

## Отчёты и отладка

- JUnit: `tests/<scenario>/reports/chainsaw-report.xml` (каталог `reports/` в `.gitignore`).
- При падении тесты собирают events и `podLogs` CA из `d8-cloud-instance-manager`.
- Полезно вручную:

```
kubectl logs -n d8-cloud-instance-manager -l app=cluster-autoscaler -c cluster-autoscaler --tail=200

kubectl get nodegroup e2e-worker-100 e2e-worker-50

kubectl get nodes -l app=e2e-autoscaler-test
```

## Быстрая диагностика «почему не стартуют»

| Симптом                               | Вероятная причина                          |
|---------------------------------------|--------------------------------------------|
| Нет deployment CA                     | Нет CloudEphemeral NG с `min < max`        |
| Assert на `worker` InstanceClass      | Нет базовой группы/класса `worker`         |
| `grep clusterapi `/` grep mcm` failed | Запущен не тот сценарий для вашего облака  |
| Timeout на логах priority             | Медленное облако, квоты, ошибки MCM/CAPI   |
| Fallback timeout                      | Долгий backoff; для Yandex заложено до 1 ч |

## Структура каталога (для правок тестов)

```
cluster-autoscaler/
├── Taskfile.yaml              # includes всех сценариев
├── chainsaw-config.yaml       # общие таймауты
└── tests/
    ├── common/manifests/      # общий deployment e2e-nginx
    ├── ca-scale-from-zero-*/  # chainsaw-test.yaml, manifests, asserts, Taskfile.yml
    └── ca-priority-fallback-*/
```
