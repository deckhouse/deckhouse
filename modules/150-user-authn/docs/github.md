Создание приложения в Github организации для аутентификации в кластере
=======

Для настройки аутентификации с помощью модуля `user-authn` необходимо в организации Github создать новое приложение.

Для этого необходимо перейти в `Settings` -> `OAuth Aps` -> `New Oauth App` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`

Полученные `Client ID` и `Client Secret` понадобятся для настройки коннектора в модуле `user-authn`.
