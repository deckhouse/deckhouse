---
title: Что делать, если DKP не получает обновления из настроенного канала?
subsystems:
  - deckhouse
lang: ru
---

- Проверьте, что настроен [нужный канал обновлений](configuration.html#проверка-текущего-канала-обновлений).
- Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

  Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде `deckhouse`. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse на узле:

  ```shell
  getent ahosts registry.deckhouse.ru
  ```

  Пример вывода:

  ```console
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse в поде `deckhouse`:

  ```shell
  d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.ru
  ```

  Пример вывода:

  ```console
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```

  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. В частности, обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде `deckhouse`). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).

{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Далее описан пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:

- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен **1** (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении же на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попробует получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.
{% endofftopic %}
