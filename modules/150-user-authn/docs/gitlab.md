Создание приложения в Gitlab для аутентификации в кластере
=======

Для настройки аутентификации с помощью модуля `user-authn` необходимо в Gitlab'е проекта создать новое приложение.

Для этого необходимо перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`.

Полученные `Application ID` и `Secret` необходимы для настройки коннктора в модуле `user-authn`.
