---
title: "Создание приложения в GitHub организации для аутентификации в кластере" 
sidebartitle: "Аутентификация в GitHub"
---

Для настройки аутентификации с помощью модуля `user-authn` необходимо в организации GitHub создать новое приложение.

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` понадобятся для настройки коннектора в модуле `user-authn`.

В том случае, если организация Github находится под управлением клиента, необходимо перейти в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и запросить подтверждение нажатием на `Send Request`. После попросить клиента подтвердить запрос, который придет к нему на email.
