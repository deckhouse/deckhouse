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

## Доступ с использованием базовой аутентификации (LDAP)

Помимо OIDC можно настроить прямой доступ к Kubernetes API с использованием базовой аутентификации (Basic Authentication, по логину и паролю). В этом случае проверка учетных данных осуществляется через LDAP-совместимую службу каталогов.

Для настройки:

1. Включите публикацию API (параметр [`publishAPI`](/modules/user-authn/configuration.html#parameters-publishapi)).
1. Настройте провайдер LDAP в модуле `user-authn` и включите в нём опцию [`enableBasicAuth: true`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth).

{% alert level="warning" %}
В кластере может быть только один провайдер с включенным параметром [`enableBasicAuth`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth).
{% endalert %}

После этого пользователи могут настроить свой `kubeconfig`, указав логин и пароль LDAP:

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://api.example.com
    # Путь к CA сертификату или insecure-skip-tls-verify: true
    certificate-authority: /path/to/ca.crt
users:
- name: ldap-user
  user:
    username: janedoe@example.com
    password: userpassword
contexts:
- name: default
  context:
    cluster: my-cluster
    user: ldap-user
current-context: default
```

## SSO по Kerberos (SPNEGO) для LDAP

Dex поддерживает аутентификацию без отображения формы ввода логина/пароля, которая реализуется с помощью механизма Kerberos (SPNEGO) для LDAP‑коннектора. Механизм работает по следующему принципу:

1. Браузер, доверяющий хосту Dex, отправляет `Authorization: Negotiate …`.
1. Dex валидирует Kerberos‑билет по keytab и пропускает форму вводу логина/пароля.
1. Dex сопоставляет principal с LDAP‑именем, получает группы и завершает OIDC‑поток.

{% alert level="info" %}
Для настройки SSO по Kerberos (SPNEGO) в LDAP должна быть учетная запись с правами только на чтение (service account). Подробнее о настройке — в разделе [«Интеграция по LDAP»](external-authentication-providers.html#интеграция-по-ldap).
{% endalert %}

Включение SSO по Kerberos (SPNEGO) для LDAP:

1. В инфраструктуре клиента должен быть задан SPN `HTTP/<fqdn-dex>` для сервисного аккаунта и сгенерирован keytab.
1. В кластере создайте секрет в неймспейсе `d8-user-authn` с ключом `krb5.keytab`.
1. В ресурсе DexProvider (тип LDAP) включите блок `spec.ldap.kerberos` и настройте в нём параметры:
   - `enabled: true`;
   - `keytabSecretName: <имя секрета>`;
   - опционально: `expectedRealm`, `usernameFromPrincipal`, `fallbackToPassword`.

Dex автоматически смонтирует keytab и начнёт принимать SPNEGO. `krb5.conf` на сервере не обязателен — билеты проверяются по keytab.

Пример настройки SSO по Kerberos (SPNEGO) для LDAP (расширение спецификации LDAP‑провайдера):

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: active-directory
spec:
  type: LDAP
  displayName: Active Directory
  ldap:
    host: ad.example.com:636
    bindDN: cn=Administrator,cn=users,dc=example,dc=com
    bindPW: admin0!
    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      username: sAMAccountName
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: uid
        groupAttr: memberUid
    kerberos:
      enabled: true
      keytabSecretName: dex-kerberos-keytab   # Секрет в неймспейсе `d8-user-authn` с ключом 'krb5.keytab'.
      expectedRealm: EXAMPLE.COM              # Опционально, проверка realm (без учёта регистра).
      usernameFromPrincipal: sAMAccountName   # localpart|sAMAccountName|userPrincipalName
      fallbackToPassword: false               # По умолчанию false; если true — при отсутствии/ошибке заголовка `Authorization: Negotiate` будет показана форма ввода логина/пароля.
```

Примечания:

- Секрет `dex-kerberos-keytab` должен находиться в неймспейсе `d8-user-authn` и содержать ключ `krb5.keytab`.
- Один под Dex может обслуживать несколько LDAP+Kerberos провайдеров. У каждого — свой keytab; `krb5.conf` не требуется (Dex проверяет билеты офлайн по keytab).
