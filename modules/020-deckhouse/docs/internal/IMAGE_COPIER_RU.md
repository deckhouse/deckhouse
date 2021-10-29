# Копировальщик образов

Мы имеем несколько редакций Deckhouse CE, EE и FE.
Клиент в какой-то момент времени может перестать пользоваться нашими услугами.
Логично было бы перевести клиента на CE редакцию, 
но клиент может активно использовать часть функционала, предоставленного в EE-редакции. 

Например, кластер развернут в Openstack. Если мы переключим клиента на CE-редакцию,
то кластер будет неработоспособен. Также после завершения оказания услуг, мы не хотим 
поставлять обновления Deckhouse. Поэтому мы должны перепушить ВСЕ наши образы в registry клиента
и переключить Deckhouse на репозиторий клиента.

Для упрощения данной задачи был написан скрипт перепушивания образов и хук, который несколько упрощает данный процесс.

# Как это работает?

Создается специальный Secret `images-copier-config` в namespace `d8-system` 
c доступами к registry, в который будут перепушиваться образы.
В этом Secret'е мы указываем конечный адрес образа (с ТЕГОМ) Deployment `deckhouse`.

> Из-за технических особенностей НЕЛЬЗЯ использовать адрес образа с суффиксом `/dev`. Например: `client.registry/repo/deckhouse/dev:test`.

Хук добавляет в Secret текущие креды registry Deckhouse и список всех образов модулей — `/deckhouse/modules/images_tags.json` 

Данный список генерируется при сборке и кладется в образ Deckhouse.
Хук запускает Job со скриптом копирования образов, монтирую Secret в контейнер Pod'а. 

Если перепушивание завершилось успешно, то хук удаляет Secret, Job и Pod.

# Последовательность действий

## Копирование образов 

- Запусти скрипт `../images/images-copier/generate-copier-secret.sh` указав креденшалы конечного registry для перепуша.
  
  Пример: 
  `REPO_USER="u" REPO_PASSWORD='Pass"Word' IMAGE_WITH_TAG="client.registry/repo/deckhouse:test" ./generate-copier-secret.sh`

  Тег ОБЯЗАТЕЛЕН!!! Тег в принципе может быть любой, например: `rock-solid-1.24.7`. 
- На STDOUT будет выведено содержимое Secret, который нужно добавить в кластер и 
  две команды для переключения registry после перепушивания.
- Добавляем Secret `kubectl create -f - <<EOF ...`
- В кластере должна появится Job'а `kubectl -n d8-system get job copy-images`
- Ждем пока все скопируется.
- Если перепушивание успешно завершилось, то Job, Pod и Secret будут удалены из кластера, и в журнале Deckhouse появится сообщение:
  `Image copier ran successfully. Cleanup`

  Команда для проверки: `kubectl logs -n d8-system deployments/deckhouse | grep "Image copier ran successfully"`
- Если перепушивание завершилось неудачно, ни Job, ни Secret, ни Pod не будут удалены, а в журнале Deckhouse будет следующее сообщение:
  `Image copier was failed. See logs into image copier job pod for additional information`

  Команда для проверки: `kubectl logs -n d8-system deployments/deckhouse | grep "Image copier was failed"`
  
  В этом случае можно посмотреть логи Job'ы, например, с помощью следующей команды:
  `kubectl -n d8-system logs jobs/copy-images`

  Перезапустить перепушивание можно добавлением/изменением аннотаций в Secret `d8-system/images-copier-config`.

Внимание! Если удалить Secret, то Job и Pod'ы будут удалены, в независимости от состояния Job'ы.

## Переключение репозитория
Когда ты запускал `generate-copier-secret.sh` скрипт, то в выводе, кроме Secret'а, были две команды:
- первая, меняет `deckhouse-registry` Secret;
- вторая, меняет имя образа для `deckhouse-controller`.

Исполни их последовательно и дождись переката всех Pod'ов. 

Посмотреть какие образы сейчас используются:
```shell
kubectl get pods --all-namespaces -o jsonpath="{.items[*].spec.containers[*].image}" |\
tr -s '[[:space:]]' '\n' |\
sort |\
uniq -c
```
