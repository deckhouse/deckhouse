---
title: "События аудита безопасности"
menuTitle: События аудита безопасности
searchable: true
description: События аудита безопасности
permalink: ru/code/documentation/admin/audit-events.html
lang: ru
weight: 50
---

Аудит безопасности — это детальный анализ вашей инфраструктуры, направленный на выявление потенциальных уязвимостей и небезопасных практик. Code помогает упростить процесс аудита с помощью событий аудита, которые позволяют отслеживать широкий спектр действий, происходящих в системе.

## Назначение и область применения

Механизм регистрации событий безопасности (события аудита) используется для:

- фиксации действий пользователей и администраторов, связанных с изменением конфигурации системы;
- регистрации инцидентов информационной безопасности;
- обеспечения возможности расследования происшествий и восстановления полной картины действий в системе.

Область применения — все уровни платформы Code, от отдельных проектов и групп до конфигурации инстанса. События фиксируются независимо от прав пользователя (при условии, что у него есть доступ к совершению действия) и сохраняются централизованно.

## Технические меры регистрации событий

Система аудита реализует следующие меры:

- **Централизованная регистрация событий** — все события фиксируются в едином журнале аудита.
- **Непрерывный сбор данных** — событие фиксируется в реальном времени по ходу выполнения бизнес-операции. Например, при входе в систему, смене адреса электронной почты пользователя или включении возможности выполнять `force push` в защищенной ветке.
- **Защита целостности** — журнал событий доступен только в режиме чтения для администраторов. Удалить или изменить события в журнале невозможно.
- **Доступ через UI и API** — просмотр и фильтрация событий возможны как в [интерфейсе администратора](#доступ-через-ui), так и [через специализированный API](#доступ-через-api).

## Сценарии использования

События аудита безопасности позволяют отследить:

- кто и когда изменил уровень доступа пользователя в проекте Code;
- случаи создания и удаления пользователей;
- признаки подозрительной активности — например, массовая смена адресов электронной почты сотрудников или удаление репозиториев;
- случаи изменения переменных окружения в CI/CD;
- случаи изменения уровня видимости проектов и групп.

События аудита помогают:

- оценивать риски и усиливать меры безопасности;
- своевременно реагировать на инциденты.

## Доступ к событиям аудита

### Доступ через UI

Чтобы получить доступ к событиям аудита, войдите в режим администратора и в боковом меню выберите пункт «Аудит событий».
Откроется таблица событий аудита безопасности.

Описание столбцов в таблице:

- **Автор** — имя пользователя, запустившего событие.
- **Событие** — системное сообщение с информацией о событии.
- **Объект** — область, к которой относится событие (название инстанса, группы, проекта, имя пользователя).
- **Цель** — сущность, которая была изменена (название проекта, защищенной ветки, CI-переменной, имя пользователя, токена, и т. д.).
- **Время события** — дата и время наступления события.

![Таблица событий аудита безопасности](/images/code/audit_events_table.png)

### Доступ через API

Deckhouse Code предоставляет следующий API-метод для получения списка событий аудита:

`POST /api/v4/admin/audit_events/search`

Допускается фильтрация по датам, текстовому поиску и типам сущностей.

{% alert level="warning" %}
Диапазон дат должен находиться в пределах одного календарного месяца. Если значения `created_after` и `created_before` относятся к разным месяцам, параметр `created_before` будет автоматически приведён к последнему дню месяца, в который входит `created_after`.
{% endalert %}

#### Параметры запроса

| Параметр         | Тип    | Обязательный | Описание                                                                                      |
|------------------|--------|--------------|-----------------------------------------------------------------------------------------------|
| `created_after`  | Строка | Нет          | Начальная дата (включительно) в формате ISO8601. По умолчанию — начало текущего месяца.       |
| `created_before` | Строка | Нет          | Конечная дата (включительно) в формате ISO8601. По умолчанию — конец текущего месяца.         |
| `q`              | Строка | Нет          | Полнотекстовый поиск по сообщению события.                                                    |
| `sort`           | Строка | Нет          | Сортировка по дате создания. Возможные значения: `created_asc`, `created_desc` (по умолчанию). |
| `entity_types`   | Массив строк  | Нет          | Список типов сущностей для фильтрации: `User`, `Project`, `Group`, `Gitlab::Audit::InstanceScope`. |

Пример запроса:

```bash
curl --request POST "https://example.com/api/v4/admin/audit_events/search" \
     --header "PRIVATE-TOKEN: <your_access_token>" \
     --header "Content-Type: application/json" \
     --data '{
       "created_after": "2025-08-01",
       "created_before": "2025-08-31",
       "q": "repository",
       "sort": "created_desc",
       "entity_types": ["Project"]
     }'
```

## Содержимое событий аудита

Каждое событие аудита содержит дату, время, данные об IP-адресе и учетной записи пользователя, а также всю необходимую информацию об области изменения, измененном объекте и о том, что именно было изменено.

## Перечень событий аудита

В таблице приведены примеры системных сообщений. В production-среде события аудита содержат полную информацию в самом сообщении или в дополнительном JSON-поле с данными.

<div class="table-wrapper" markdown="0">
<table class="supported_versions" markdown="0" style="table-layout: fixed">
<thead>
<tr>
<th>Название</th>
<th>Системное сообщение</th>
<th>Назначение</th>
<th>Отслеживаемые атрибуты</th>
</tr>
</thead>
<tbody>
<tr>
<td><code style="word-break: break-all; white-space: normal;">2fa_login_failed</code></td>
<td>User 2fa login failed</td>
<td>Обнаружена неудачная попытка входа с двухфакторной аутентификацией.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_approved</code></td>
<td>User access was approved</td>
<td>Пользователю одобрен запрос на доступ в инстанс.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_token_created</code></td>
<td>Project/Group access token created</td>
<td>Создан токен доступа для проекта или группы.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_token_revoked</code></td>
<td>Project/Group access token revoked</td>
<td>Токен доступа был отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">added_gpg_key</code></td>
<td>Added new gpg key to user</td>
<td>Пользователь добавил новый GPG-ключ.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">added_ssh_key</code></td>
<td>User added new ssh key</td>
<td>Пользователь добавил новый SSH-ключ.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_created</code></td>
<td>Application was created</td>
<td>Создано приложение (OAuth или интеграция).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_deleted</code></td>
<td>Application deleted</td>
<td>Приложение было удалено.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_secret_renew</code></td>
<td>Application secret renew</td>
<td>Обновлён секрет приложения.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_updated</code></td>
<td>Application Updated</td>
<td>Изменены параметры приложения.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_cd_job_token_removed_from_allowlist</code></td>
<td>Disallow group to use job token</td>
<td>В проекте ограничено использование CI/CD Job Token определённой группой.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_cd_job_token_added_to_allowlist</code></td>
<td>Allow group to use job token</td>
<td>В проекте разрешено использование CI/CD Job Token определённой группой.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_created</code></td>
<td>Ci variable <code>#{key}</code> created</td>
<td>Создана новая переменная CI/CD.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_deleted</code></td>
<td>Ci variable <code>#{key}</code> deleted</td>
<td>Удалена переменная CI/CD.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_updated</code></td>
<td>Ci variable updated (Value, Protected)</td>
<td>Изменено значение или параметры защиты переменной CI/CD.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_created</code></td>
<td>Deploy key added</td>
<td>Создан новый ключ развёртывания (deploy key) для проекта/инстанса.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_deleted</code></td>
<td>Deploy key was deleted</td>
<td>Удалён ключ развертывания.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_disabled</code></td>
<td>Deploy key disabled</td>
<td>Ключ развёртывания отключён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_enabled</code></td>
<td>Deploy key enabled</td>
<td>Ключ развёртывания включён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_created</code></td>
<td>Deploy token created</td>
<td>Создан токен развёртывания (deploy token) для доступа к данным.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_deleted</code></td>
<td>Deploy token deleted</td>
<td>Удалён токен развёртывания.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_revoked</code></td>
<td>Deploy token revoked</td>
<td>Токен развёртывания был отозван пользователем или системой.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_created</code></td>
<td>Created feature flag with description</td>
<td>Создан новый флаг функций (feature flag).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_deleted</code></td>
<td>Feature flag was deleted</td>
<td>Флаг функций был удалён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_updated</code></td>
<td>Feature flag was updated</td>
<td>Обновлены параметры флага функций.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_created</code></td>
<td>Group was created</td>
<td>Создана новая группа.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_export_created</code></td>
<td>Group file export was created</td>
<td>Создан файл экспорта группы.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_created</code></td>
<td>Invited group to group</td>
<td>В группу приглашена другая группа через групповую ссылку.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_deleted</code></td>
<td>Revoked group from group</td>
<td>Доступ группы, приглашенной по ссылке, был отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_updated</code></td>
<td>Group access changed</td>
<td>Изменены параметры доступа группы через групповую ссылку.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_created</code></td>
<td>Invited group to project</td>
<td>В проект приглашена группа через групповую ссылку.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_deleted</code></td>
<td>Revoked group from project</td>
<td>Доступ группы к проекту был отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_updated</code></td>
<td>Group access for project changed</td>
<td>Изменены параметры доступа группы к проекту.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_updated</code></td>
<td>Group updated (visibility, 2FA grace period)</td>
<td>Изменения в настройках группы (видимость, безопасность, лимиты, политика доступа).</td>
<td><ul>
<li><code>repository_size_limit</code></li>
<li><code>two_factor_grace_period</code></li>
<li><code>lfs_enabled</code></li>
<li><code>membership_lock</code></li>
<li><code>path</code></li>
<li><code>require_two_factor_authentication</code></li>
<li><code>request_access_enabled</code></li>
<li><code>shared_runners_minutes_limit</code></li>
<li><code>share_with_group_lock</code></li>
<li><code>mentions_disabled</code></li>
<li><code>max_personal_access_token_lifetime</code></li>
<li><code>visibility_level</code></li>
<li><code>name</code></li>
<li><code>description</code></li>
<li><code>project_creation_level</code></li>
<li><code>default_branch_protected</code></li>
<li><code>seat_control</code></li>
<li><code>duo_features_enabled</code></li>
<li><code>prevent_forking_outside_group</code></li>
<li><code>allow_mfa_for_subgroups</code></li>
<li><code>default_branch_name</code></li>
<li><code>resource_access_token_creation_allowed</code></li>
<li><code>new_user_signups_cap</code></li>
<li><code>show_diff_preview_in_email</code></li>
<li><code>enabled_git_access_protocol</code></li>
<li><code>runner_registration_enabled</code></li>
<li><code>allow_runner_registration_token</code></li>
<li><code>emails_enabled</code></li>
<li><code>service_access_tokens_expiration_enforced</code></li>
<li><code>enforce_ssh_certificates</code></li>
<li><code>disable_personal_access_tokens</code></li>
<li><code>remove_dormant_members</code></li>
<li><code>remove_dormant_members_period</code></li>
<li><code>prevent_sharing_groups_outside_hierarchy</code></li>
<li><code>default_branch_protection_defaults</code></li>
<li><code>wiki_access_level</code></li>
</ul></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">impersonation_initiated</code></td>
<td>User root impersonated another user</td>
<td>Администратор начал сессию от имени другого пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">impersonation_stopped</code></td>
<td>User root stopped impersonation</td>
<td>Администратор завершил сессию от имени другого пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">instance_settings_updated</code></td>
<td>Instance settings updated: Signup enabled turned on</td>
<td>Изменены глобальные настройки инстанса.</td>
<td>Все поля с настройками инстанса, кроме зашифрованных.</td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">login_failed</code></td>
<td>Attempt to login failed</td>
<td>Неудачная попытка входа в систему.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">manually_trigger_housekeeping</code></td>
<td>Housekeeping task</td>
<td>Запущена задача обслуживания репозитория вручную.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_created</code></td>
<td>New member access granted</td>
<td>Пользователю предоставлен доступ (роль) к группе или проекту.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_destroyed</code></td>
<td>Member access revoked</td>
<td>Доступ пользователя к проекту или группе отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_updated</code></td>
<td>Member access updated</td>
<td>Изменены права или срок действия доступа пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_closed_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> closed by project bot</td>
<td>Запрос на merge закрыт системным ботом проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_created_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> created by project bot</td>
<td>Запрос на merge создан системным ботом проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_merged_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> merged by project bot</td>
<td>Запрос на merge обработан системным ботом проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_reopened_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> reopened by project bot</td>
<td>Запрос на merge переоткрыт системным ботом проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">omniauth_login_failed</code></td>
<td>Omniauth login failed for <code>#{user}</code> <code>#{provider}</code></td>
<td>Ошибка входа через внешний OAuth/Omniauth-провайдер.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">password_reset_failed</code></td>
<td>Password reset failed</td>
<td>Неудачная попытка сброса пароля пользователем.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">personal_access_token_issued</code></td>
<td>Personal access token issued</td>
<td>Выпущен новый токен доступа (personal access token).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">personal_access_token_revoked</code></td>
<td>Personal access token revoked</td>
<td>Токен доступа был отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">pipeline_deleted</code></td>
<td>Pipeline deleted</td>
<td>Конвейер CI/CD был удалён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_blobs_removal</code></td>
<td>Project blobs removed</td>
<td>Массовое удаление объектов (blobs) из проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_created</code></td>
<td>Project was created</td>
<td>Создан новый проект.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_default_branch_changed</code></td>
<td>Project default branch updated</td>
<td>Изменена ветка по умолчанию в проекте.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_export_created</code></td>
<td>Project export created</td>
<td>Создан файл экспорта проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_feature_updated</code></td>
<td>Project features updated</td>
<td>Изменены уровни доступа к функциям проекта (issues, wiki и т. д.).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_setting_updated</code></td>
<td>Project settings updated</td>
<td>Изменены шаблоны merge commit и squash commit.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_text_replacement</code></td>
<td>Project text replaced</td>
<td>В проекте выполнена массовая замена текста.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_topic_changed</code></td>
<td>Project topic changed</td>
<td>Изменена тема проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_updated</code></td>
<td>Project updated (name, namespace)</td>
<td>Изменены настройки проекта (имя, неймспейс, политики).</td>
<td><ul>
<li><code>name</code></li>
<li><code>packages_enabled</code></li>
<li><code>reset_approvals_on_push</code></li>
<li><code>path</code></li>
<li><code>merge_requests_author_approval</code></li>
<li><code>merge_requests_disable_committers_approval</code></li>
<li><code>only_allow_merge_if_all_discussions_are_resolved</code></li>
<li><code>only_allow_merge_if_pipeline_succeeds</code></li>
<li><code>require_password_to_approve</code></li>
<li><code>disable_overriding_approvers_per_merge_request</code></li>
<li><code>repository_size_limit</code></li>
<li><code>project_namespace_id</code></li>
<li><code>namespace_id</code></li>
<li><code>printing_merge_request_link_enabled</code></li>
<li><code>resolve_outdated_diff_discussions</code></li>
<li><code>merge_requests_ff_only_enabled</code></li>
<li><code>merge_requests_rebase_enabled</code></li>
<li><code>remove_source_branch_after_merge</code></li>
<li><code>merge_requests_template</code></li>
<li><code>visibility_level</code></li>
<li><code>builds_access_level</code></li>
<li><code>container_registry_access_level</code></li>
<li><code>environments_access_level</code></li>
<li><code>feature_flags_access_level</code></li>
<li><code>forking_access_level</code></li>
<li><code>infrastructure_access_level</code></li>
<li><code>issues_access_level</code></li>
<li><code>merge_requests_access_level</code></li>
<li><code>metrics_dashboard_access_level</code></li>
<li><code>monitor_access_level</code></li>
<li><code>operations_access_level</code></li>
<li><code>package_registry_access_level</code></li>
<li><code>pages_access_level</code></li>
<li><code>releases_access_level</code></li>
<li><code>repository_access_level</code></li>
<li><code>requirements_access_level</code></li>
<li><code>security_and_compliance_access_level</code></li>
<li><code>snippets_access_level</code></li>
<li><code>wiki_access_level</code></li>
<li><code>merge_commit_template</code></li>
<li><code>squash_commit_template</code></li>
<li><code>runner_registration_enabled</code></li>
<li><code>show_diff_preview_in_email</code></li>
<li><code>selective_code_owner_removals</code></li>
</ul></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_branch_created</code></td>
<td>Protected branch created</td>
<td>Создана защищённая ветка.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_branch_deleted</code></td>
<td>Protected branch was deleted</td>
<td>Удалена защищённая ветка.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_branch_updated</code></td>
<td>Protected branch was updated:</td>
<td>Обновлены правила защищённой ветки.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_created</code></td>
<td>Protected tag created</td>
<td>Создан защищённый тег.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_deleted</code></td>
<td>Protected tag was deleted</td>
<td>Удалён защищённый тег.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_updated</code></td>
<td>Protected tag updated:</td>
<td>Обновлены правила защищённого тега.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">removed_gpg_key</code></td>
<td>Removed gpg key from user</td>
<td>Удалён GPG-ключ пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">removed_ssh_key</code></td>
<td>User removed ssh key</td>
<td>Удалён SSH-ключ пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">requested_password_reset</code></td>
<td>User requested password change</td>
<td>Пользователь запросил сброс пароля.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">revoked_gpg_key</code></td>
<td>Revoked gpg key from user</td>
<td>GPG-ключ пользователя был отозван.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unban_user</code></td>
<td>User was unban</td>
<td>Пользователь разблокирован (unban).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unblock_user</code></td>
<td>User was unblocked</td>
<td>С пользователя снята блокировка (unblock).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_access_locked</code></td>
<td>User access locked</td>
<td>Учётная запись пользователя заблокирована.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_access_unlocked</code></td>
<td>User access unlocked</td>
<td>Учётная запись пользователя разблокирована.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_activated</code></td>
<td>User was activated</td>
<td>Учётная запись пользователя активирована.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_banned</code></td>
<td>User was banned</td>
<td>Пользователь забанен.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_blocked</code></td>
<td>User was blocked</td>
<td>Учётная запись пользователя заблокирована.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_created</code></td>
<td>User was created</td>
<td>Создан новый пользователь.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_deactivated</code></td>
<td>User was deactivated</td>
<td>Учётная запись пользователя деактивирована.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_destroyed</code></td>
<td>User was destroyed</td>
<td>Учётная запись пользователя удалена.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_email_updated</code></td>
<td>User email updated</td>
<td>Изменён адрес электронной почты пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_logged_in</code></td>
<td>User logged in</td>
<td>Успешный вход пользователя.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_password_updated</code></td>
<td>Password updated</td>
<td>Пароль пользователя изменён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_rejected</code></td>
<td>User was rejected</td>
<td>Учётная запись пользователя отклонена (например, при регистрации).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_removed_two_factor</code></td>
<td>Two factor disabled</td>
<td>Пользователь отключил двухфакторную аутентификацию.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_settings_updated</code></td>
<td>User settings updated</td>
<td>Обновлены настройки профиля пользователя.</td>
<td><ul>
<li><code>name</code></li>
<li><code>public_email</code></li>
<li><code>otp_secret</code></li>
<li><code>otp_required_for_login</code></li>
<li><code>admin</code></li>
<li><code>private_profile</code></li>
</ul></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_signup</code></td>
<td>User was registered</td>
<td>Пользователь зарегистрирован.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_switched_to_admin_mode</code></td>
<td>User switched to admin mode</td>
<td>Пользователь включил режим администратора.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_username_updated</code></td>
<td>Username updated</td>
<td>Изменено имя пользователя (username).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">webhook_created</code></td>
<td>Webhook was created</td>
<td>Создан вебхук для проекта, группы или инстанса.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">webhook_destroyed</code></td>
<td>System hook removed</td>
<td>Вебхук удалён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_deleted</code></td>
<td>Group was deleted</td>
<td>Группа удалена.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_deleted</code></td>
<td>Project was deleted</td>
<td>Проект удалён.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">logout</code></td>
<td>User logged out</td>
<td>Пользователь вышел из системы.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unauthenticated_session</code></td>
<td>Redirected to login</td>
<td>Система перенаправила неаутентифицированного пользователя на страницу входа.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runners_bulk_deleted</code></td>
<td>CI runner bulk deleted: Errors:</td>
<td>Массовое удаление CI runners.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_registered</code></td>
<td>CI runner created via API</td>
<td>Регистрация CI runner через API.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_unregistered</code></td>
<td>CI runner unregistered</td>
<td>Отмена регистрации CI runner.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_token_reset</code></td>
<td>CI runner registration token reset</td>
<td>Сброшен токен CI runner.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_assigned_to_project</code></td>
<td>CI runner assigned to project</td>
<td>Runner был привязан к проекту.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_unassigned_from_project</code></td>
<td>CI runner unassigned from project</td>
<td>Runner был отвязан от проекта.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_created</code></td>
<td>CI runner created via UI</td>
<td>Runner был создан через графический интерфейс.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">package_registry_package_published</code></td>
<td><code style="word-break: break-all; white-space: normal;">#{name}</code> package version <code>#{version}</code> has been published</td>
<td>В реестре пакетов опубликован новый пакет.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">package_registry_package_deleted</code></td>
<td>package version <code>#{package.version}</code> has been deleted</td>
<td>Пакет удалён из реестра пакетов.</td>
<td></td>
</tr>
</tbody>
</table>
</div>
