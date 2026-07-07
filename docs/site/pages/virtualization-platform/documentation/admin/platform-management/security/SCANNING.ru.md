---
title: Сканирование контейнерных образов на уязвимости
permalink: ru/virtualization-platform/documentation/admin/platform-management/security/scanning.html
lang: ru
---

Deckhouse Virtualization Platform (DVP) предоставляет встроенное средство для автоматического поиска уязвимостей
в контейнерных образах на основе проекта [Trivy](https://github.com/aquasecurity/trivy).

## Поиск уязвимостей

DVP запускает регулярное сканирование всех контейнерных образов, используемых в подах кластера.
Проверка выполняется каждые 24 часа и охватывает:

- известные уязвимости (CVE) в используемых образах;
- соответствие CIS-стандартам (compliance-проверки).

Для сканирования используются как [публичные базы уязвимостей](https://github.com/aquasecurity/trivy-db/tree/main/pkg/vulnsrc),
так и обогащённые данные из [Astra Linux](https://astralinux.ru/), [ALT Linux](https://www.basealt.ru/products) и [РЕД ОС](https://redos.red-soft.ru/product/server/).

## Сканирование в пространствах имён

По умолчанию сканируется только пространство имён `default`.

Чтобы выполнить сканирование в конкретном пространстве имён, добавьте для него лейбл `security-scanning.deckhouse.io/enabled=""`.

Как только в кластере обнаруживается хотя бы одно пространство имён с указанным лейблом, сканирование пространства имён `default` прекращается.
Чтобы снова включить сканирование для пространства имён `default`, добавьте для него лейбл следующей командой:

```shell
d8 k label namespace default security-scanning.deckhouse.io/enabled=""
```

В текущей версии функциональности ограничения перечня ресурсов для сканирования в пространстве имён не предусмотрено.  
DVP сканирует **все ресурсы**, находящиеся в пространстве имён, помеченном лейблом `security-scanning.deckhouse.io/enabled=""`.

## Повторное сканирование

Сканирование происходит автоматически каждые 24 часа согласно следующему алгоритму:

1. В пространстве имён c каждым просканированным ресурсом создаётся объект VulnerabilityReport.
1. Этот объект содержит аннотацию `trivy-operator.aquasecurity.github.io/report-ttl`,
   которая определяет срок жизни отчёта (по умолчанию - `24h`).
1. По истечении этого срока объект VulnerabilityReport удаляется и сканирование запускается повторно.

### Принудительный повтор сканирования

Чтобы запустить повторное сканирование ресурса вручную, воспользуйтесь одним из двух способов:

- Перезапишите аннотацию `trivy-operator.aquasecurity.github.io/report-ttl`, указав короткий срок жизни отчёта.

  Пример команды:

  ```shell
  d8 k annotate VulnerabilityReport -n <NAMESPACE> <REPORT_NAME> trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
  ```

- Удалите объект VulnerabilityReport из пространства имён, где находится просканированный ресурс.

  Пример команды:

  ```shell
  d8 k delete VulnerabilityReport -n <NAMESPACE> <REPORT_NAME>
  ```
