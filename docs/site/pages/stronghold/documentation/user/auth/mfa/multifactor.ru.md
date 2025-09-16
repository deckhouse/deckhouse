---
title: "MULTIFACTOR Ldap Adapter"
permalink: ru/stronghold/documentation/user/auth/mfa/multifactor.html
lang: ru
---

MULTIFACTOR Ldap Adapter — LDAP proxy-сервер, разработанный и поддерживаемый компанией МУЛЬТИФАКТОР. Он используется для двухфакторной защиты пользователей в приложениях, использующих LDAP-аутентификацию.
Система обеспечивает многофакторную аутентификацию и контроль доступа для любых удалённых подключений: RDP, VPN, VDI, SSH и других.

## Настройка LDAP Adapter

### Схема работы

Stronghold может осуществлять двухфакторную аутентификацию пользователей из каталога LDAP или Active Directory:

- Пользователь подключается к Stronghold, вводит логин и пароль;
- Stronghold по протоколу LDAP подключается к компоненту [MULTIFACTOR Ldap Adapter](https://multifactor.ru/docs/ldap-adapter/ldap-adapter/);
- Компонент проверяет логин и пароль пользователя в Active Directory или другом LDAP-каталоге и запрашивает второй фактор аутентификации;
- Пользователь подтверждает запрос доступа выбранным способом аутентификации.

### Настройка MULTIFACTOR

1. Зайдите в [систему управления MULTIFACTOR](https://admin.multifactor.ru/account/login), в разделе «Ресурсы» создайте новое LDAP приложение.
  После создания будут доступны два параметра: `NAS Identifier` и `Shared Secret`, они потребуются для последующих шагов.
1. Загрузите и установите [MULTIFACTOR Ldap Adapter](https://multifactor.ru/docs/ldap-adapter/ldap-adapter/).

### Запуск LDAP Adapter в Kubernetes

Для запуска воспользуйтесь образом `multifactor-ldap-adapter:3.0.7` и следующим манифестом:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-adapter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ldap-adapter
  template:
    metadata:
      labels:
        app: ldap-adapter
    spec:
      containers:
      - image: registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7
        name: ldap-adapter
        volumeMounts:
        - mountPath: /opt/multifactor/ldap/multifactor-ldap-adapter.dll.config
          name: config
          subPath: multifactor-ldap-adapter.dll.config
      volumes:
      - configMap:
          defaultMode: 420
          name: ldap-adapter
        name: config
---
apiVersion: v1
kind: Service
metadata:
  name: ldap-adapter
spec:
  ports:
  - port: 389
    protocol: TCP
    targetPort: 389
  selector:
    app: ldap-adapter
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ldap-adapter
data:
  multifactor-ldap-adapter.dll.config: |
    <?xml version="1.0" encoding="utf-8"?>
    <configuration>
      <configSections>
        <section name="UserNameTransformRules" type="MultiFactor.Ldap.Adapter.Configuration.UserNameTransformRulesSection, multifactor-ldap-adapter" />
      </configSections>
      <appSettings>
        <add key="adapter-ldap-endpoint" value="0.0.0.0:389"/>
        <add key="ldap-server" value="ldap://ldap.example.com"/>
        <add key="ldap-service-accounts" value="CN=admin,DC=example,DC=com"/>
        <add key="ldap-base-dn" value="ou=Users,dc=example,dc=com"/>
        <add key="multifactor-api-url" value="https://api.multifactor.ru" />
        <add key="multifactor-nas-identifier" value="YOUR-NAS-IDENTIFIER" />
        <add key="multifactor-shared-secret" value="YOUR-NAS-SECRET" />
        <add key="logging-level" value="Debug"/>
      </appSettings>
    </configuration>
```

В конфигурации укажите адрес своего LDAP-сервера и значения `multifactor-nas-identifier` и `multifactor-shared-secret` из панели управления MULTIFACTOR.

Доступные образы:
- на базе Ubuntu 24.04 `registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7`
- на базе Alpine 3.22 `registry.deckhouse.ru/stronghold/multifactor/multifactor-ldap-adapter:3.0.7-alpine`

## Настройка Stronghold

Для настройки Stronghold создайте и сконфигурируйте метод аутентификации `ldap`, где в качестве сервера укажите адрес `ldap-adapter`. Если для запуска адаптера вы использовали манифест из примера выше, то нужно указать адрес `ldap://ldap-adapter.default.svc`:

```shell
d8 stronghold auth enable ldap
d8 stronghold write auth/ldap/config url="ldap://ldap-adapter.default.svc" \
   binddn="cn=admin,dc=example,dc=com" bindpass="Password-1" \
   userdn="ou=Users,dc=example,dc=com" groupdn="ou=Groups,dc=example,dc=com" \
   username_as_alias=true
```

## Тестирование с помощью локального сервера openldap

Ниже приведен пример манифеста, с помощью которого можно запустить сервис OpenLDAP в Kubernetes для целей тестирования:

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openldap
spec:
  replicas: 1
  selector:
    matchLabels:
      app: openldap
  template:
    metadata:
      labels:
        app: openldap
    spec:
      containers:
      - env:
        - name: LDAP_ADMIN_DN
          value: cn=admin,dc=example,dc=com
        - name: LDAP_ROOT
          value: dc=example,dc=com
        - name: LDAP_ADMIN_USERNAME
          value: admin
        - name: LDAP_ADMIN_PASSWORD
          value: Password-1
        image: bitnami/openldap:2.6.10
        name: openldap
---
apiVersion: v1
kind: Service
metadata:
  name: openldap
spec:
  ports:
  - name: p389
    port: 389
    protocol: TCP
    targetPort: 1389
  selector:
    app: openldap
```

После того как запустите контейнер, создайте пользователя (в качестве примера приведено создание пользователя `alice` с паролем `D3mo-Passw0rd`).

Сначала выполните вход в контейнер openldap:

```shell
d8 k exec svc/openldap -it -- bash
```

Создайте пользователя с помощью следующих команд:

```shell
cd /tmp
cat << EOF > create_entries.ldif
dn: uid=alice,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: person
objectClass: top
cn: Alice
sn: User
userPassword: D3mo-Passw0rd
EOF

ldapadd -H ldap://openldap -cxD "cn=admin,dc=example,dc=com" \
        -w "Password-1" -f "create_entries.ldif"
```

Можете выполнить вход под пользователем `alice` с паролем `D3mo-Passw0rd`. В [панели управления MULTIFACTOR](https://admin.multifactor.ru/account/login)
в разделе `Пользователи` будет создан пользователь `alice`, для которого можно назначить второй фактор.
Далее будет требоваться его подтверждение при каждом входе в Stronghold.
Помимо аудит-логов на стороне Stronghold подтверждение второго фактора будет фиксироваться также на стороне MULTIFACTOR.
