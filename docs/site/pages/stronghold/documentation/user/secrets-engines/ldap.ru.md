---
title: "Механизм секретов LDAP"
permalink: ru/stronghold/documentation/user/secrets-engines/ldap.html
lang: ru
description: >-
  The LDAP secret engine manages LDAP entry passwords.
---

{% raw %}
Механизм секретов LDAP обеспечивает управление учетными данными LDAP, а также динамическое создание учетных данных. Он поддерживает интеграцию с реализациями протокола LDAP v3, включая OpenLDAP, Active Directory и IBM Resource Access Control Facility (RACF).

Механизм секретов имеет три основные функции:

- [Управление статическими учетными данными](#static-roles)
- [Управление динамическими учетными данными](#dynamic-roles)
- [Ротация паролей для списов учетных записей](#rotation)

## Настройка

Включите механизм секретов LDAP:

```sh
d8 stronghold secrets enable ldap
```

   По умолчанию подключение произойдет по пусти `ldap`. Для подключения по другому пути используйте аргумент `-path`.

Настройте учетные данные, которые Stronghold использует для подключения к LDAP для генерации паролей:

```sh
d8 stronghold write ldap/config \
    binddn=$USERNAME \
    bindpass=$PASSWORD \
    url=ldaps://138.91.247.105
```

   Примечание: рекомендуется создать отдельную учетную запись специально для Stronghold.

Ротируйте пароль, чтобы он харнился только в Stronghold:

```sh
d8 stronghold write -f ldap/rotate-root
```

   Примечание: получить сгенерированный пароль после ротации в Stronghold невозможно.

### Схемы LDAP {#schemas}

Механизм секретов LDAP поддерживает три различные схемы:

- `openldap` (по умолчанию)
- `racf`
- `ad`

#### OpenLDAP

По умолчанию механизм секретов LDAP предполагает, что пароль для учетной записи хранится в `userPassword`.
Существует множество классов объектов, которые имеют поле `userPassword`, включая, например:

- `organization`
- `organizationalUnit`
- `organizationalRole`
- `inetOrgPerson`
- `person`
- `posixAccount`

#### Resource access control facility (RACF)

Для управления системой безопасности IBM Resource Access Control Facility (RACF) механизм секретов должен быть настроен на использование схемы `racf`.

Для поддержки RACF генерируемые пароли должны состоять из 8 символов или меньше. Длина пароля может быть настроена с помощью политики паролей:

```bash
d8 stronghold write ldap/config \
 binddn=$USERNAME \
 bindpass=$PASSWORD \
 url=ldaps://138.91.247.105 \
 schema=racf \
 password_policy=racf_password_policy
```

#### Active directory (AD)

Для управления паролями в Active Directory механизм секретов должен быть настроен на использование схемы `ad`.

```bash
d8 stronghold write ldap/config \
 binddn=$USERNAME \
 bindpass=$PASSWORD \
 url=ldaps://138.91.247.105 \
 schema=ad
```

### Статические роли {#static-roles}

#### Настройка

Настройте статическую роль, которая сопоставляет имя в Stronghold с записью в LDAP.
   Настройки ротации паролей будут управляться этой ролью.

```sh
d8 stronghold write ldap/static-role/lf-edge\
    dn='uid=lf-edge,ou=users,dc=lf-edge,dc=com' \
    username='stronghold'\
    rotation_period="24h"
```

Запросите учетные данные для роли "stronghold":

```sh
d8 stronghold read ldap/static-cred/lf-edge
```

### Ротация паролей

Управление паролями может осуществляться двумя способами:

- автоматическая ротация по времени
- ручная ротация

### Автоматическая ротация паролей

Пароли будут автоматически сменяться в зависимости от `rotation_period`, настроенного в статической роли (минимум 5 секунд). При запросе учетных данных для статической роли в ответе будет указано время до следующей ротации (`ttl`).

В настоящее время авторотация поддерживается только для статических ролей. Учетная запись `binddn`, используемая Stronghold, должна быть ротирована с помощью вызова `rotate-root`, чтобы сгенерировать пароль, который будет знать только Stronghold.

### Ручная ротация

Пароли статической роли могут быть ротированы вручную с помощью вызова `rotate-role`. При ручной ротации период ротации начинается заново.

### Удаление статических ролей

При удалении статической роли пароли не сменяются. Пароль должен быть ротирован вручную перед удалением роли или отзывом доступа к статической роли.

### Динамические роли {#dynamic-roles}

#### Настройка

Динамическую роль можно настроить с помощью вызова `/role/:role_name`:

```bash
d8 stronghold write ldap/role/dynamic-role \
  creation_ldif=@/path/to/creation.ldif \
  deletion_ldif=@/path/to/deletion.ldif \
  rollback_ldif=@/path/to/rollback.ldif \
  default_ttl=1h \
  max_ttl=24h
```

{% endraw %}

{% alert level="warning" %}
Аргумент `rollback_ldif` необязателен, но рекомендуется. Операции, указанные в `rollback_ldif` будут выполнены, если создание по какой-либо причине завершится неудачей. Это поможет гарантировать, что все объекты будут удалены в случае неудачи.
{% endalert %}

{% raw %}
Чтобы сгенерировать учетные данные, выполните:

```bash
d8 stronghold read ldap/creds/dynamic-role
```

Пример вывода:

```console
Key                    Value
---                    -----
lease_id               ldap/creds/dynamic-role/HFgd6uKaDomVMvJpYbn9q4q5
lease_duration         1h
lease_renewable        true
distinguished_names    [cn=v_token_dynamic-role_FfH2i1c4dO_1611952635,ou=users,dc=learn,dc=example]
password               xWMjkIFMerYttEbzfnBVZvhRQGmhpAA0yeTya8fdmDB3LXDzGrjNEPV2bCPE9CW6
username               v_token_testrole_FfH2i1c4dO_1611952635
```

Поле `distinguished_names` представляет собой массив DN, созданных на основе `creation_ldif`. Если включено более одной записи LDIF, в это поле будут включены DN из каждого из . Каждая запись в этом поле соответствует одному LDIF-заявлению. Дедупликации не происходит, и порядок сохраняется.

### Записи LDIF

Управление учетными записями пользователей осуществляется с помощью записей LDIF. Записи LDIF могут представлять собой base64-кодированную версию строки LDIF. Строка будет разобрана и проверена на соответствие синтаксису LDIF. Хороший справочник по правильному синтаксису LDIF можно найти [здесь](https://ldap.com/ldif-the-ldap-data-interchange-format/).

Некоторые важные моменты, которые следует помнить при создании записей LDIF:

- В конце строк не должно быть пробелов.
- Каждый блок `modify` должен предваряться пустой строкой
- Несколько модификаций для `dn` могут быть определены в одном блоке `modify`. Каждая модификация должна завершаться одним тире (`-`)

### Active directory (AD)

Для Active Directory есть несколько дополнительных деталей, которые важно помнить:

Чтобы программно создать пользователя в AD, сначала нужно выполнить добавление (`add`) объекта пользователя, и только затем изменить (`modufy`) этого пользователя, чтобы указать пароль и включить учетную запись.

- Пароли в AD задаются с помощью поля `unicodePwd`. Перед ним должны стоять два (2) двоеточия (`::`).
- При программной установке пароля в AD должны быть соблюдены следующие критерии:

  - Пароль должен быть заключен в двойные кавычки (`""`)
  - Пароль должен быть в [формате `UTF16LE`](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/6e803168-f140-4d23-b2d3-c3a8ab5917d2)
  - Пароль должен быть `base64`-кодирован
  - Дополнительные сведения можно найти [здесь](https://docs.microsoft.com/en-us/troubleshoot/windows-server/identity/set-user-password-with-ldifde)

- После того как пароль пользователя установлен, его можно включить. Для этого в AD используется поле `userAccountControl`:
  - Чтобы включить учетную запись, установите `userAccountControl` в `512`
  - Скорее всего, вы также захотите отключить истечение срока действия пароля AD для этой динамической учетной записи пользователя. Значение `userAccountControl` для этого: `65536`
  - Флаги `userAccountControl` являются кумулятивными, поэтому, чтобы установить оба вышеуказанных флага, сложите два значения (`512 + 65536 = 66048`): установите `userAccountControl` в `66048`.
  - Более подробную информацию о флагах `userAccountControl` можно получить [здесь](https://docs.microsoft.com/en-us/troubleshoot/windows-server/identity/useraccountcontrol-manipulate-account-properties#property-flag-descriptions)

`sAMAccountName` - распространенное поле при работе с пользователями AD. Оно используется для обеспечения совместимости с устаревшими системами Windows NT и имеет ограничение в 20 символов. Имейте это в виду при определении шаблона `username_template`. Дополнительные сведения см. на [здесь](https://docs.microsoft.com/en-us/windows/win32/adschema/a-samaccountname).

Поскольку стандартный `username_template` длиннее 20 символов и соответствует шаблону `v_{{.DisplayName}}_{{.RoleName}}_{{random 10}}_{{unix_time}}`, мы рекомендуем настроить `username_template` в конфигурации роли, чтобы генерировать учетные записи с именами менее 20 символов.

AD не позволяет напрямую изменять атрибут `memberOf` пользователя. Атрибут `member` группы и атрибут `memberOf` пользователя являются [связанными атрибутами](https://docs.microsoft.com/en-us/windows/win32/ad/linked-attributes). Связанные атрибуты представляют собой пары прямая ссылка/обратная ссылка, причем прямая ссылка может быть изменена. В случае членства в группе AD атрибут `member` группы является прямой ссылкой. Чтобы добавить вновь созданного динамического пользователя в группу, нам также необходимо отправить запрос `modify` в нужную группу и добавить туда пользователя.

#### Пример LDIF для Active directory

Различные параметры `*_ldif` представляют собой шаблоны, использующие язык [go template](https://golang.org/pkg/text/template/). Полный пример LDIF для создания учетной записи пользователя Active Directory приведен здесь для справки:

```ldif
dn: CN={{.Username}},OU=Stronghold,DC=adtesting,DC=lab
changetype: add
objectClass: top
objectClass: person
objectClass: organizationalPerson
objectClass: user
userPrincipalName: {{.Username}}@adtesting.lab
sAMAccountName: {{.Username}}

dn: CN={{.Username}},OU=Stronghold,DC=adtesting,DC=lab
changetype: modify
replace: unicodePwd
unicodePwd::{{ printf "%q" .Password | utf16le | base64 }}
-
replace: userAccountControl
userAccountControl: 66048
-

dn: CN=test-group,OU=Stronghold,DC=adtesting,DC=lab
changetype: modify
add: member
member: CN={{.Username}},OU=Stronghold,DC=adtesting,DC=lab
-
```

## Ротация паролей для списков учетных записей {#rotation}

Stronghold может автоматически менять пароли для группы учетных записей. Операция по ротации пароля может быть выполнена вручную, или Stronghold выполнит ее, когда истечет TTL от предыдущей смены.

Функционал работает с различными [схемами](#schemas), включая OpenLDAP, Active Directory и RACF. В следующем примере рассмотрим вариант с Active Directory.

Сначала нам нужно включить механизм секретов LDAP и указать ему, как подключиться к серверу AD.

Пример:

```shell-session
$ d8 stronghold secrets enable ldap
Success! Enabled the ad secrets engine at: ldap/

$ d8 stronghold write ldap/config \
    binddn=$USERNAME \
    bindpass=$PASSWORD \
    url=ldaps://138.91.247.105 \
    userdn='dc=example,dc=com'
```

Далее настроим список учетных записей, для которых требуется выполнинять ротацию пароля.

```shell-session
d8 stronghold write ldap/library/accounting-team \
    service_account_names=fizz@example.com,buzz@example.com \
    ttl=10h \
    max_ttl=20h \
    disable_check_in_enforcement=false
```

В этом примере имена учетных записей служб `fizz@example.com` и `buzz@example.com` уже были созданы на удаленном сервере AD. `ttl` - это время, через которое Stronghold повторно выполнить ротацию пароля учетной записи. `max_ttl` - максимальное время, которое может действовать пароль после ротации. По умолчанию значения обоихпараметров равны `24h`. Также по умолчанию учетная запись службы должна быть зарегистрирована тем же субъектом Stronghold или клиентским токеном, который выполняет ротацию. Однако если такое поведение вызывает проблемы, установите `disable_check_in_enforcement=true`.

После создания списка учетных записей вы можете в любой момент просмотреть их статус.

Пример:

```shell-session
d8 stronghold read ldap/library/accounting-team/status
```

Пример вывода:

```shell-session
Key                 Value
---                 -----
buzz@example.com    map[available:true]
fizz@example.com    map[available:true]
```

Для ротации паролей, выполните команду:

```shell-session
d8 stronghold write -f ldap/library/accounting-team/check-out
```

Пример вывода:

```shell-session
Key                     Value
---                     -----
lease_id                ldap/library/accounting-team/check-out/EpuS8cX7uEsDzOwW9kkKOyGW
lease_duration          10h
lease_renewable         true
password                ?@09AZKh03hBORZPJcTDgLfntlHqxLy29tcQjPVThzuwWAx/Twx4a2ZcRQRqrZ1w
service_account_name    fizz@example.com
```

Если стандартное значение `ttl` больше, чем требуется, установите более короткое время с помощью команды:

```shell-session
d8 stronghold write ldap/library/accounting-team/check-out ttl=30m
```

Пример вывода:

```shell-session
Key                     Value
---                     -----
lease_id                ldap/library/accounting-team/check-out/gMonJ2jB6kYs6d3Vw37WFDCY
lease_duration          30m
lease_renewable         true
password                ?@09AZerLLuJfEMbRqP+3yfQYDSq6laP48TCJRBJaJu/kDKLsq9WxL9szVAvL/E1
service_account_name    buzz@example.com
```

Вы можете продлить аренду паролей для набора учетных записей.

```shell-session
d8 stronghold lease renew ldap/library/accounting-team/check-out/0C2wmeaDmsToVFc0zDiX9cMq
```

Пример вывода:

```shell-session
Key                Value
---                -----
lease_id           ldap/library/accounting-team/check-out/0C2wmeaDmsToVFc0zDiX9cMq
lease_duration     10h
lease_renewable    true
```

В этом случае текущиие пароли для аккаунтов будет жить дольше, так как мы отрочим выполнение ротации.

## Политика паролей LDAP

Механизм секретов LDAP не хэширует и не шифрует пароли перед изменением значений в LDAP. Такое поведение может привести к тому, что в LDAP будут храниться пароли в открытом виде.

Чтобы избежать хранения паролей в открытом виде, на сервере LDAP должна быть настроена политика паролей LDAP (ppolicy, не путать с политикой паролей Stronghold). Политика ppolicy может применять такие правила, как хэширование паролей по умолчанию.

Ниже приведен пример политики паролей LDAP для применения хэширования для `dc=example,dc=com`:

```console
dn: cn=module{0},cn=config
changetype: modify
add: olcModuleLoad
olcModuleLoad: ppolicy

dn: olcOverlay={2}ppolicy,olcDatabase={1}mdb,cn=config
changetype: add
objectClass: olcPPolicyConfig
objectClass: olcOverlayConfig
olcOverlay: {2}ppolicy
olcPPolicyDefault: cn=default,ou=pwpolicies,dc=example,dc=com
olcPPolicyForwardUpdates: FALSE
olcPPolicyHashCleartext: TRUE
olcPPolicyUseLockout: TRUE
```

{% endraw %}
