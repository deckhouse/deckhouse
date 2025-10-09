---
title: "События аудита безопасности"
menuTitle: События аудита безопасности
force_searchable: true
description: События аудита безопасности
permalink: ru/code/documentation/admin/audit-events.html
lang: ru
weight: 50
---

Аудит безопасности — это детальный анализ вашей инфраструктуры, направленный на выявление потенциальных уязвимостей и небезопасных практик. Code помогает упростить процесс аудита с помощью событий аудита, которые позволяют отслеживать широкий спектр действий, происходящих в системе.

## Назначение и область применения

Механизм регистрации событий безопасности (События аудита) используется для:

- фиксации действий пользователей и администраторов, связанных с изменением конфигурации системы;
- регистрации инцидентов информационной безопасности;
- обеспечения возможности расследования происшествий и восстановления полной картины действий в системе.

Область применения: все уровни платформы Code — от отдельных проектов и групп до конфигурации инстанса. События фиксируются независимо от прав пользователя (при условии, что у него есть доступ к совершению действия) и сохраняются централизованно.

## Технические меры регистрации событий

Система аудита реализует следующие меры:

- **Централизованная регистрация событий** — все события фиксируются в едином журнале аудита.
- **Непрерывный сбор данных** — Событие фиксируется в реальном времени во время выполнения бизнес операции. Например (вход в систему, изменения емейла пользователя, включение возможности force push в защищенную ветку)
- **Защита целостности** — журнал событий доступен только в режиме чтения для администраторов. Удаление или изменение событий невозможно.
- **Доступ через UI и API** — просмотр и фильтрация событий возможны как в интерфейсе администратора, так и через специализированный API.

## Сценарии использования событий аудита безопасности

- Отслеживание, кто и когда изменил уровень доступа пользователя в проекте Code.
- Отслеживание случаев создания и удаления пользователей.
- Выявление подозрительной активности — например, массового изменения email-адресов сотрудников или удаления репозиториев.
- Отслеживание изменений переменных окружения в CI/CD.
- Отслеживание изменений уровня видимости проектов и групп.

События аудита помогают:

- оценивать риски и усиливать меры безопасности;
- своевременно реагировать на инциденты;

## Доступ к событиям аудита безопасности

Чтобы получить доступ к событиям аудита, войдите в режим администратора и в боковом меню выберите пункт «Аудит событий».

Описание столбцов в таблице:

- Автор - Указывается пользователь который запустил событие
- Событие - Системное сообщение с информацией о событие
- Объект - Область (Инстанс, Пользователь, Группа, Проект) к которой относится событие
- Цель - Сущность которая была изменена. Проект, Пользователь, Защищенная ветка, токен, CI переменная итд.
- Время события – Дата и время наступления события

![Пример](/images/code/audit_events1.png)

## Доступ к событиям аудита через API

Code также предоставляет API-метод для получения списка событий аудита с возможностью фильтрации и сортировки.

**Метод:**  
`POST /api/v4/admin/audit_events/search`

**Описание:**  
Возвращает список событий аудита. Допускается фильтрация по датам, текстовому поиску и типам сущностей.

{% alert level="warning" %}
Диапазон дат должен находиться в пределах одного календарного месяца. Если значения `created_after` и `created_before` относятся к разным месяцам, параметр `created_before` будет автоматически приведён к последнему дню месяца, в который входит `created_after`.
{% endalert %}

### Параметры запроса

| Параметр         | Тип    | Обязательный | Описание                                                                                      |
|------------------|--------|--------------|-----------------------------------------------------------------------------------------------|
| `created_after`  | string | Нет          | Начальная дата (включительно) в формате ISO8601. По умолчанию — начало текущего месяца.       |
| `created_before` | string | Нет          | Конечная дата (включительно) в формате ISO8601. По умолчанию — конец текущего месяца.         |
| `q`              | string | Нет          | Полнотекстовый поиск по сообщению события.                                                    |
| `sort`           | string | Нет          | Сортировка по дате создания. Возможные значения: `created_asc`, `created_desc` (по умолчанию). |
| `entity_types`   | array  | Нет          | Список типов сущностей для фильтрации: `User`, `Project`, `Group`, `Gitlab::Audit::InstanceScope`. |

**Пример запроса:**

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

## Что содержат события аудита безопасности

События аудита содержит дату, время, ip адресс пользователя, учетную запись пользователя а также всю информацию для идентификации в какой области произошли изменения, какой объект был изменен и что именно было изменено.

## События аудита безопасности

В таблице приведены примеры сообщений. Реальные события аудита содержат полную информацию в самом сообщении или в дополнительное json поле с данными.

| Название                          | Сообщение в системе                                        | Назначение                                                                                   | Аудируемые аттрибуты |
|-------------------------------|---------------------------------------------------|----------------------------------------------------------------------------------------------|------------|
| 2fa_login_failed              | User 2fa login failed                             | Фиксируется неудачная попытка входа с двухфакторной аутентификацией.                         |            |
| access_approved               | User access was approved                          | Пользователю одобрен запрос на доступ в Инстанс.                                                      |            |
| access_token_created          | Project/Group access token created                | Создан токен доступа для проекта или группы.                                                 |            |
| access_token_revoked          | Project/Group access token revoked                | Токен доступа был отозван.                                                                   |            |
| added_gpg_key                 | Added new gpg key to user                         | Пользователь добавил новый GPG-ключ.                                                         |            |
| added_ssh_key                 | User added new ssh key                            | Пользователь добавил новый SSH-ключ.                                                         |            |
| application_created           | Application was created                           | Создано приложение (OAuth или интеграция).                                                   |            |
| application_deleted           | Application deleted                               | Приложение было удалено.                                                                      |            |
| application_secret_renew      | Application secret renew                          | Обновлён секрет (secret) приложения.                                                         |            |
| application_updated           | Application Updated                               | Изменены параметры приложения.                                                               |            |
| ci_cd_job_token_removed_from_allowlist     | Disallow group to use job token             | В проекте ограничено использование CI/CD Job Token определённой группой.                     |            |
| ci_cd_job_token_added_to_allowlist | Allow group to use job token               | В проекте разрешено использование CI/CD Job Token определённой группой.                      |            |
| ci_variable_created           | Ci variable #{key} created                        | Создана новая переменная CI/CD.                                                              |            |
| ci_variable_deleted           | Ci variable #{key} deleted                        | Удалена переменная CI/CD.                                                                    |            |
| ci_variable_updated           | Ci variable updated (Value, Protected)            | Изменено значение или параметры защиты переменной CI/CD.                                      |            |
| deploy_key_created            | Deploy key added                                  | Создан новый deploy key для проекта/инстанса.                                                |            |
| deploy_key_deleted            | Deploy key was deleted                            | Удалён deploy key.                                                                           |            |
| deploy_key_disabled           | Deploy key disabled                               | Deploy key отключён.                                                                         |            |
| deploy_key_enabled            | Deploy key enabled                                | Deploy key включён.                                                                          |            |
| deploy_token_created          | Deploy token created                              | Создан deploy token для доступа к данным.                                                    |            |
| deploy_token_deleted          | Deploy token deleted                              | Удалён deploy token.                                                                         |            |
| deploy_token_revoked          | Deploy token revoked                              | Deploy token был отозван пользователем или системой.                                          |            |
| feature_flag_created          |Created feature flag with description                                                 | Создан новый Feature Flag.                                                       |            |
| feature_flag_deleted          | Feature flag was deleted                                                 | Feature Flag был удалён.                                                                     |            |
| feature_flag_updated          | Feature flag was updated                                                 | Обновлены параметры Feature Flag.                                                            |            |
| group_created                 | Group was created                                 | Создана новая группа.                                                                        |            |
| group_export_created          | Group file export was created                     | Выполнен экспорт группы.                                                              |            |
| group_invite_via_group_link_created  | Invited group to group                      | В группу приглашена другая группа через групповую ссылку.                                    |            |
| group_invite_via_group_link_deleted  | Revoked group from group                     | Доступ другой группы по ссылке был отозван.                                                  |            |
| group_invite_via_group_link_updated  | Group access changed                        | Изменены параметры доступа группы через групповую ссылку.                                    |            |
| group_invite_via_project_group_link_created | Invited group to project                | В проект приглашена группа через групповую ссылку.                                           |            |
| group_invite_via_project_group_link_deleted | Revoked group from project               | Доступ группы к проекту был отозван.                                                         |            |
| group_invite_via_project_group_link_updated | Group access for project changed         | Изменены параметры доступа группы к проекту.                                                 |            |
| group_updated                 | Group updated (visibility, 2FA grace period)      | Изменения в настройках группы (видимость, безопасность, лимиты, политика доступа).            | repository_size_limit, two_factor_grace_period, lfs_enabled, membership_lock, path, require_two_factor_authentication, request_access_enabled, shared_runners_minutes_limit, share_with_group_lock, mentions_disabled, max_personal_access_token_lifetime, visibility_level, name, description, project_creation_level, default_branch_protected, seat_control, duo_features_enabled, prevent_forking_outside_group, allow_mfa_for_subgroups, default_branch_name, resource_access_token_creation_allowed, new_user_signups_cap, show_diff_preview_in_email, enabled_git_access_protocol, runner_registration_enabled, allow_runner_registration_token, emails_enabled, service_access_tokens_expiration_enforced, enforce_ssh_certificates, disable_personal_access_tokens, remove_dormant_members, remove_dormant_members_period, prevent_sharing_groups_outside_hierarchy, default_branch_protection_defaults, wiki_access_level |
| impersonation_initiated       | User root impersonated another user               | Администратор начал сессию от имени другого пользователя.                                    |            |
| impersonation_stopped         | User root stopped impersonation                   | Администратор завершил сессию от имени другого пользователя.                                 |            |
| instance_settings_updated     | Instance settings updated: Signup enabled turned on                         | Изменены глобальные настройки инстанса.                                                      |   Все поля с настройками инстанса кроме зашифрованных         |
| login_failed                  | Attempt to login failed                           | Неудачная попытка входа в систему.                                                           |            |
| manually_trigger_housekeeping | Housekeeping task                                 | Запущена задача обслуживания репозитория вручную.                                            |            |
| member_permissions_created    | New member access granted                         | Пользователю предоставлен доступ (роль) к группе или проекту.                                |            |
| member_permissions_destroyed  | Member access revoked                             | Доступ пользователя к проекту или группе отозван.                                           |            |
| member_permissions_updated    | Member access updated                             | Изменены права или срок действия доступа пользователя.                                       |            |
| merge_request_closed_by_project_bot | Merge request #{merge_request.title} closed by project bot                                           | Merge request закрыт системным ботом проекта.                                                |            |
| merge_request_created_by_project_bot | Merge request #{merge_request.title} created by project bot                                          | Merge request создан системным ботом проекта.                                                |            |
| merge_request_merged_by_project_bot | Merge request #{merge_request.title} merged by project bot                                           | Merge request смержен системным ботом проекта.                                               |            |
| merge_request_reopened_by_project_bot | Merge request #{merge_request.title} reopened by project bot                                         | Merge request переоткрыт системным ботом проекта.                                            |            |
| omniauth_login_failed         | Omniauth login failed for #{user} #{provider}                                                 | Ошибка входа через внешний OAuth/Omniauth провайдер.                                         |            |
| password_reset_failed         | Password reset failed                                                 | Неудачная попытка сброса пароля пользователем.                                               |            |
| personal_access_token_issued  | Personal access token issued                      | Выпущен новый personal access token.                                                         |            |
| personal_access_token_revoked | Personal access token revoked                     | Personal access token был отозван.                                                           |            |
| pipeline_deleted              | Pipeline deleted                                                 | Конвейер CI/CD был удалён.                                                                   |            |
| project_blobs_removal         | Project blobs removed                             | Массовое удаление объектов (blobs) из проекта.                                               |            |
| project_created               | Project was created                               | Создан новый проект.                                                                         |            |
| project_default_branch_changed| Project default branch updated                     | Изменена ветка по умолчанию в проекте.                                                       |            |
| project_export_created        | Project export created                            | Выполнен экспорт проекта.                                                                    |            |
| project_feature_updated       | Project features updated                          | Изменены уровни доступа к функциям проекта (issues, wiki и т.д.).                            |            |
| project_setting_updated       | Project settings updated                          | Изменены шаблоны merge commit и squash commit.                                               |            |
| project_text_replacement      | Project text replaced                             | В проекте выполнена массовая замена текста.                                                  |            |
| project_topic_changed         | Project topic changed                             | Изменена тема проекта.                                                                       |            |
| project_updated               | Project updated (name, namespace)                 | Изменены настройки проекта (имя, namespace, политики).                                       | name, packages_enabled, reset_approvals_on_push, path, merge_requests_author_approval, merge_requests_disable_committers_approval, only_allow_merge_if_all_discussions_are_resolved, only_allow_merge_if_pipeline_succeeds, require_password_to_approve, disable_overriding_approvers_per_merge_request, repository_size_limit, project_namespace_id, namespace_id, printing_merge_request_link_enabled, resolve_outdated_diff_discussions, merge_requests_ff_only_enabled, merge_requests_rebase_enabled, remove_source_branch_after_merge, merge_requests_template, visibility_level, builds_access_level, container_registry_access_level, environments_access_level, feature_flags_access_level, forking_access_level, infrastructure_access_level, issues_access_level, merge_requests_access_level, metrics_dashboard_access_level, monitor_access_level, operations_access_level, package_registry_access_level, pages_access_level, releases_access_level, repository_access_level, requirements_access_level, security_and_compliance_access_level, snippets_access_level, wiki_access_level, merge_commit_template, squash_commit_template, runner_registration_enabled, show_diff_preview_in_email, selective_code_owner_removals |
| protected_branch_created      | Protected branch created                                                 | Создана защищённая ветка.                                                                    |            |
| protected_branch_deleted      | Protected branch was deleted                                                 | Удалена защищённая ветка.                                                                    |            |
| protected_branch_updated      | Protected branch was updated:                                                 | Обновлены правила защищённой ветки.                                                          |            |
| protected_tag_created         | Protected tag created                                                 | Создан защищённый тег.                                                                       |            |
| protected_tag_deleted         | Protected tag was deleted                                                 | Удалён защищённый тег.                                                                       |            |
| protected_tag_updated         | Protected tag updated:                                                  | Обновлены правила защищённого тега.                                                          |            |
| removed_gpg_key               | Removed gpg key from user                         | Удалён GPG-ключ пользователя.                                                                |            |
| removed_ssh_key               | User removed ssh key                              | Удалён SSH-ключ пользователя.                                                                |            |
| requested_password_reset      | User requested password change                    | Пользователь запросил сброс пароля.                                                          |            |
| revoked_gpg_key               | Revoked gpg key from user                         | GPG-ключ пользователя был отозван.                                                           |            |
| unban_user                    | User was unban                                    | Пользователь разблокирован (unban).                                                          |            |
| unblock_user                  | User was unblocked                                | С пользователя снята блокировка (unblock).                                                   |            |
| user_access_locked            | User access locked                                | Учётная запись пользователя заблокирована.                                                   |            |
| user_access_unlocked          | User access unlocked                              | Учётная запись пользователя разблокирована.                                                  |            |
| user_activated                | User was activated                                | Учётная запись пользователя активирована.                                                    |            |
| user_banned                   | User was banned                                   | Пользователь забанен.                                                                        |            |
| user_blocked                  | User was blocked                                  | Учётная запись пользователя заблокирована.                                                   |            |
| user_created                  | User was created                                  | Создан новый пользователь.                                                                   |            |
| user_deactivated              | User was deactivated                              | Учётная запись пользователя деактивирована.                                                  |            |
| user_destroyed                | User was destroyed                                | Учётная запись пользователя удалена.                                                         |            |
| user_email_updated            | User email updated                                | Изменён адрес электронной почты пользователя.                                                |            |
| user_logged_in                | User logged in                                    | Успешный вход пользователя.                                                                  |            |
| user_password_updated         | Password updated                                                 | Пароль пользователя изменён.                                                                 |            |
| user_rejected                 | User was rejected                                 | Учётная запись пользователя отклонена (например, при регистрации).                           |            |
| user_removed_two_factor       | Two factor disabled                               | Пользователь отключил двухфакторную аутентификацию.                                          |            |
| user_settings_updated         | User settings updated                             | Обновлены настройки профиля пользователя.                                                    | name, public_email, otp_secret, otp_required_for_login, admin, private_profile |
| user_signup                   | User was registered                               | Пользователь зарегистрирован.                                                                |            |
| user_switched_to_admin_mode   | User switched to admin mode                       | Пользователь включил режим администратора.                                                   |            |
| user_username_updated         | Username updated                                  | Изменено имя пользователя (username).                                                        |            |
| webhook_created               | Webhook was created                               | Создан webhook для проекта/группы/инстанса.                                                  |            |
| webhook_destroyed              | System hook removed                              | Webhook удалён.                                                                              |            |
| group_deleted                 | Group was deleted                                                 | Группа удалена.                                                                              |            |
| project_deleted               | Project was deleted                                                | Проект удалён.                                                                               |            |
| logout                        | User logged out                                   | Пользователь вышел из системы.                                                               |            |
| unauthenticated_session       | Redirected to login                               | Система перенаправила неаутентифицированного пользователя на страницу входа.                 |            |
| ci_runners_bulk_deleted       | CI runner bulk deleted: Errors:                                                 | Массовое удаление CI runners.                                                                |            |
| ci_runner_registered          | CI runner created via API                                                 | Регистрация CI runner.                                                                       |            |
| ci_runner_unregistered        | CI runner unregistered                                                 | Отмена регистрации CI runner.                                                                |            |
| ci_runner_token_reset         | CI runner registration token reset                                                 | Сброшен токен CI runner.                                                                     |            |
| ci_runner_assigned_to_project | CI runner assigned to project                                                 | Runner был привязан к проекту.                                                               |            |
| ci_runner_unassigned_from_project | CI runner unassigned from project                                             | Runner был отвязан от проекта.                                                               |            |
| ci_runner_created             | CI runner created via UI                                                 | Runner был создан.                                                                           |            |
| package_registry_package_published | #{name} package version #{version} has been published                                            | В реестр пакетов опубликован новый пакет.                                                    |            |
| package_registry_package_deleted   | package version "#{package.version} has been deleted                                            | Пакет удален из реестра пакетов.                                                             |            |
