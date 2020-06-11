---
title: "Создание приложения в GitLab для аутентификации в кластере" 
sidebar: modules-user-authn
permalink: modules/150-user-authn/gitlab.html
hide_sidebar: false
---

Для настройки аутентификации с помощью модуля `user-authn` необходимо в GitLab проекта создать новое приложение.

Для этого необходимо перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`.

Полученные `Application ID` и `Secret` необходимы для настройки коннектора в модуле `user-authn`.
