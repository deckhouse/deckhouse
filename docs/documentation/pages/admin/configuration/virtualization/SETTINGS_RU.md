---
title: "Параметры модуля виртуализации"
permalink: ru/admin/configuration/virtualization/settings.html
description: "Параметры ModuleConfig модуля virtualization: включение и выключение модуля, версия конфигурации и настройки Ingress для загрузки образов."
search: параметры модуля, ModuleConfig, настройки виртуализации, ingressClass
lang: ru
---

Конфигурация модуля `virtualization` задаётся в ресурсе [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig). Ниже приведён пример базовой настройки, в которой указаны класс Ingress-контроллера, хранилище образов и подсеть для виртуальных машин:

{% tabs moduleconfig %}

{% tab "В командной строке" %}

Примените манифест с нужными параметрами:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    ingressClass: nginx # опциональный параметр
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
          storageClassName: rv-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Deckhouse» → «Модули».
1. Из списка выберите модуль `virtualization`.
1. В открывшемся окне выберите вкладку «Конфигурация».
1. Чтобы отобразить настройки, нажмите переключатель «Дополнительные настройки».
1. Задайте параметры. Названия полей формы соответствуют названиям параметров в YAML.
1. Нажмите кнопку «Сохранить».

{% endtab %}

{% endtabs %}

## Включение и выключение модуля

За состояние модуля отвечает параметр [`.spec.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-enabled). Значение `true` включает модуль, значение `false` выключает его.

Выключение модуля останавливает все системные компоненты, которые создают и запускают виртуальные машины (ВМ), поэтому по умолчанию модуль выключить нельзя.
Чтобы это стало возможным, добавьте на [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) `virtualization` аннотацию `modules.deckhouse.io/allow-disabling` со значением `true`.

Перед выключением подготовьте кластер:

1. Удалите все ресурсы модуля, включая виртуальные машины, диски и образы.
1. Убедитесь, что в кластере не осталось активных ресурсов:

   ```shell
   d8 k get virtualization -A
   d8 k get virtualization-cluster
   ```

После этого отредактируйте [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) `virtualization`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
  annotations:
    modules.deckhouse.io/allow-disabling: "true"
spec:
  enabled: false
  version: 1
  settings:
    # Укажите существующие настройки.
```

{% alert level="danger" %}
Если ресурсы модуля не удалены, выключение может привести к потере данных.
{% endalert %}

## Версия конфигурации

Параметр [`.spec.version`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-version) определяет версию схемы настроек. Структура параметров может меняться между версиями, актуальные значения приведены в [настройках модуля](/modules/virtualization/configuration.html).

## Настройки Ingress

Образы виртуальных машин загружаются в кластер через [Ingress-контроллер](/modules/ingress-nginx/), класс которого определяет параметр [`.spec.settings.ingressClass`](/modules/virtualization/configuration.html#parameters-ingressclass).
Указывать его необязательно, и если параметр не задан, модуль использует глобальное значение из конфигурации Deckhouse Platform (DP).
Задавайте его только тогда, когда для загрузки образов нужен отдельный Ingress-контроллер.

Пример:

```yaml
spec:
  settings:
    ingressClass: nginx
```

{% alert level="info" %}
Большие образы виртуальных машин по медленному каналу связи загружаются долго, и перезапуск или обновление Ingress-контроллера прерывает такую загрузку.
Чтобы этого избежать, увеличьте тайм-аут завершения рабочих процессов в ресурсе [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  config:
    worker-shutdown-timeout: 1800s  # 30 минут или более при необходимости
```

{% endalert %}
