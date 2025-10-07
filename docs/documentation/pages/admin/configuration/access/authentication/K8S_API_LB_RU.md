---
title: "Доступ к Kubernetes API через балансировщик трафика"
permalink: ru/admin/configuration/access/authentication/k8s-api-lb.html
description: "Настройка аутентифицированного доступа к Kubernetes API через балансировщик трафика в Deckhouse Kubernetes Platform. Безопасный доступ kubectl через Ingress-контроллер с аутентификацией."
lang: ru
---

С DKP можно использовать аутентификацию при доступе к Kubernetes API. В этом случае, пользователь в веб-интерфейсе kubeconfig DKP может сгенерировать конфигурацию для `kubectl`, для безопасного доступа к Kubernetes API через балансировщик трафика (Ingress-контроллер).

Чтобы настроить доступ, выполните следующие шаги:

1. Включите публикацию Kubernetes API. Для этого установите [параметр `publishAPI.enabled: true`](/modules/user-authn/configuration.html#parameters-publishapi-enabled) в настройках модуля `user-authn` или с помощью веб-интерфейса администратора Deckhouse.

   Пример конфигурации модуля:

   ```yaml
   spec:
     enabled: true
     version: 2
     settings:
       publishAPI:
         enabled: true
   ```

1. Откройте веб-интерфейс [kubeconfig](../../../../user/web/kubeconfig.html). Веб-интерфейс для генерации kubeconfig в DKP активируется автоматически после включения параметра `publishAPI` в модуле `user-authn`. Этот веб-интерфейс доступен по URL:

   ```console
   https://kubeconfig.<publicDomainTemplate>
   ```

   Например, если `publicDomainTemplate`: `%s.kube.my`, то URL будет `https://kubeconfig.kube.my`.

1. Сгенерируйте конфигурацию `kubectl`. После авторизации в интерфейсе kubeconfig пользователь получит набор команд для настройки `kubectl`. Эти команды можно скопировать и вставить в консоль. Аутентификация будет производиться по OIDC-токену, выданному Dex. При поддержке провайдером функции продления сессии конфигурация будет включать `refresh token`, что позволит продлевать доступ без повторной аутентификации.

1. Настройте несколько точек подключения к API. В [конфигурации модуля `user-authn`](/modules/user-authn/configuration.html#parameters-kubeconfiggenerator) можно задать несколько точек подключения (kube-apiserver) с описанием и CA-сертификатами для каждой. Это может понадобиться, если кластер доступен через разные сети — например, VPN или публичный IP:

   ```yaml
   settings:
     kubeconfigGenerator:
     - id: direct
       masterURI: https://159.89.5.247:6443
       description: "Direct access to kubernetes API"
   ```

## Как работает защита доступа к Kubernetes API

В Deckhouse Kubernetes Platform вы можете безопасно опубликовать Kubernetes API наружу с помощью Ingress-контроллера, сохранив контроль над доступом. Публикация API и настройка аутентификации осуществляется через [модуль `user-authn`](/modules/user-authn/). Вы можете настроить:

- список доверенных IP-адресов или сетей, которым разрешён доступ;
- список групп пользователей, которые имеют право аутентификации;
- Ingress-контроллер, через который будет осуществляться доступ.

Для настройки:

1. Включите публикацию API, как в примере выше.
1. Настройте ограничения доступа. В [конфигурации модуля](/modules/user-authn/configuration.html) можно указать:
   - список сетевых адресов, которым разрешён доступ (`allowedSourceRanges`);
   - список групп пользователей, которым разрешено подключение к Kubernetes API (`allowedUserGroups`);
   - выбор Ingress-контроллера, через который будет работать публикация (`ingressClass`).
1. Используйте веб-интерфейс kubeconfig. Пользователи смогут получить безопасный доступ к API через kubeconfig, сгенерированный в веб-интерфейсе (`https://kubeconfig.<publicDomainTemplate>`). Этот kubeconfig будет содержать OIDC-токен и настройки подключения через Ingress.

Что будет настроено автоматически при включении публикации API:

- Deckhouse сам настроит необходимые аргументы для kube-apiserver;
- будет сгенерирован сертификат CA и добавлен в kubeconfig;
- будет настроен вход через Dex с поддержкой OIDC.
