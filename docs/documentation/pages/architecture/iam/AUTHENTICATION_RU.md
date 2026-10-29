---
title: "Аутентификация"
permalink: ru/architecture/iam/authentication.html
lang: ru
search: аутентификация, kubeconfig, аутентификация пользователей, контроль доступа, доступ к API Kubernetes
description: Особенности аутентификации в Deckhouse Platform.
---

## Подключение к API Kubernetes с помощью сгенерированного kubeconfig

![Схема взаимодействия при подключении к API Kubernetes с помощью сгенерированного kubeconfig](../../images/user-authn/kubeconfig_dex.svg)

1. **Инициализация**. kube-apiserver получает конфигурацию OIDC-провайдера (в данном случае — Dex): `issuer` и JWKS-эндпоинт для валидации токенов. В Kubernetes 1.30 и новее он получает её через сервис Dex внутри кластера.

1. **Генерация kubeconfig**. [Веб-интерфейс Deckhouse](../../glossary.html#web-ui) Platform (DP) формирует kubeconfig, в котором сохраняются `ID token` и `refresh token`. Этот файл используется утилитой `kubectl` или другими клиентами Kubernetes.

1. **Аутентификация при обращении к API**. При получении запроса с `ID token`, kube-apiserver проверяет его подпись, используя ключи, полученные с JWKS-эндпоинта. Затем проверяются значения claim'ов `iss` (issuer) и `aud` (audience) токена на соответствие конфигурации сервера.

## Защита Dex от подбора логина и пароля

Dex ограничивает частоту неудачных попыток входа с одного IP-адреса на эндпоинтах проверки пароля. Ограничение настраивается параметром [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit) и по умолчанию выключено.

Кроме того, Dex блокирует учётную запись после неудачных попыток входа. Блокировка настраивается параметром [`passwordPolicy.lockout`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout). Она действует для локальных пользователей, а для пользователей LDAP и Crowd — если их типы указаны в [`applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors).

## Аутентификация с помощью DexAuthenticator

![Как работает аутентификация с помощью DexAuthenticator](../../images/user-authn/dex_login.svg)

1. **Процесс входа через Dex**. В большинстве случаев Dex перенаправляет пользователя на страницу входа внешнего провайдера (например, GitHub, Okta, Keycloak) и ожидает, что после успешной аутентификации пользователь будет возвращён на адрес `/callback`.
   Однако для провайдеров, таких как LDAP или Atlassian Crowd, этот механизм не работает. В таких случаях пользователь вводит логин и пароль в форму Dex, и сам Dex выполняет проверку через API соответствующего провайдера.

1. **Хранение токенов и сессий**. DexAuthenticator устанавливает cookie с полным `refresh token`, а не выдает короткоживущий ticket, как это делается с `ID token`. Это связано с тем, что Redis, используемый в DexAuthenticator, не сохраняет данные на диск.
   Если в Redis не найден `ID token` по ticket, пользователь может получить новый `ID token`, предъявив `refresh token`, сохранённый в cookie.

1. **Передача токена в приложение**. DexAuthenticator добавляет HTTP-заголовок `Authorization` с `ID token` из Redis.
   Это поведение может быть необязательным для некоторых приложений, таких как Upmeter, где механизмы авторизации реализованы иначе.
   Однако для приложений вроде Kubernetes Dashboard это обязательное поведение, так как Dashboard использует `ID token` для доступа к Kubernetes API от имени пользователя.

## Расширения Dex в DP

В DP используется Dex с дополнительными возможностями:

* группы [статических пользователей](../../glossary.html#static-user) (объект [Group](/modules/user-authn/cr.html#group));
* двухфакторная аутентификация (2FA) статических пользователей (параметр [`staticUsers2FA`](/modules/user-authn/configuration.html#parameters-staticusers2fa));
* парольная политика для статических пользователей: сложность, история паролей, ротация и блокировка после неудачных попыток входа (параметр [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy));
* принудительная смена пароля при следующем входе после сброса пароля администратором;
* блокировка после неудачных попыток входа для пользователей LDAP и Atlassian Crowd (параметр [`passwordPolicy.lockout.applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors));
* ограничение частоты неудачных попыток входа с одного IP-адреса (параметр [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit));
* блокировка и разблокировка пользователей с помощью объекта [UserOperation](/modules/user-authn/cr.html#useroperation), в том числе без парольной политики;
* списки разрешённых пользователей и групп в [DexClient](/modules/user-authn/cr.html#dexclient) и [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator), которые Dex проверяет при входе и при каждом запросе с refresh-токеном;
* ограничение идентичностей, которые может утверждать провайдер (параметр [`spec.allowedIdentities`](/modules/user-authn/cr.html#dexprovider-v1-spec-allowedidentities) DexProvider);
* вход по Kerberos (SPNEGO) через провайдер LDAP без формы ввода пароля, с проверкой билетов из заголовка `Authorization: Negotiate` по keytab сервиса;
* refresh-токены для провайдера SAML;
* собственный сертификат CA для провайдера GitLab (параметр [`gitlab.rootCAData`](/modules/user-authn/cr.html#dexprovider-v1-spec-gitlab-rootcadata)).

## Отказоустойчивый режим

В отказоустойчивом режиме ([`highAvailability`](/modules/user-authn/configuration.html#parameters-highavailability)) Dex и другие компоненты модуля `user-authn` запускаются в нескольких репликах, поэтому вход продолжает работать при отказе одного пода. DexAuthenticator использует ту же настройку, если в нём не задан собственный параметр [`spec.highAvailability`](/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability).
