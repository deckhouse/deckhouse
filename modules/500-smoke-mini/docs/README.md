---
title: "Модуль smoke-mini"
---

Модуль предназначен для мониторинга состояния кластера методом его постоянного *smoke-тестирования*.

Модуль запускает 3 `StatefulSet` использующих `PV` и каждый имеющий 1 реплику, со специальным приложением, поднимающим http сервер и предоставляющим API для выполнения тестов. Ресурсы одного из `StatefulSet` перешедуливаются раз в 10 минут на случайные узлы. 

Модуль по умолчанию **включен**.

### Функционал тестирования
* `/` – return 200
* `/error` – return 500
* `/api` – проверяет доступ к кубовому API (запрашивается информация по поду из которого выполняется запрос `/api/v1/namespaces/d8-smoke-mini/pods/<POD_NAME>`)
* `/dns` – проверяет работу кластерного dns (выполняет резолв домена `kubernetes.default`)
* `/disk` – проверяет, что может создать и удалить файл
* `/neighbor` – проверяет, есть ли доступ к "соседу" по HTTP
* `/prometheus` – проверяет, что может отправить запрос в прометей `/api/v1/metadata?metric=prometheus_build_info`

### Параметры:
* `storageClass` — имя storageClass'а, который использовать.
    * Если не указано — используется StorageClass существующей PVC, а если PVC пока нет — используется или `global.storageClass`, или `global.discovery.defaultStorageClass`, а если и их нет — данные сохраняются в emptyDir.
    * Если указать `false` — будет форсироваться использование emptyDir'а.
* `ingressClass` — класс ingress контроллера, который используется для smoke-mini.
    * Опциональный параметр, по умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой типа сертификата использовать для smoke-mini.
    * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
    * `mode` — режим работы HTTPS:
        * `Disabled` — в данном режиме smoke-mini будет работать только по http;
        * `CertManager` — smoke-mini будет работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
        * `CustomCertificate` — smoke-mini будет работать по https используя сертификат из namespace `d8-system`;
        * `OnlyInURI` — smoke-mini будет работать по http (подразумевая, что перед ним стоит внешний https балансер, который терминирует https).
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для smoke-mini (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для smoke-mini (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
        * По умолчанию `false`.
