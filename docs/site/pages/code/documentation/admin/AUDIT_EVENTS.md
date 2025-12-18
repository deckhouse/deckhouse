---
title: "Audit events"
menuTitle: Audit events
searchable: true
description: Audit events
permalink: en/code/documentation/admin/audit-events.html
lang: en
weight: 50
---

Security audit is a detailed analysis of your infrastructure aimed at identifying potential vulnerabilities and unsafe practices.  
Code simplifies the auditing process with audit events, which allow you to track a wide range of activities happening in the system.

## Purpose and scope

The security event logging mechanism (audit events) is used to:

- Record user and administrator actions related to system configuration changes.
- Register information security incidents.
- Provide the ability to investigate incidents and reconstruct a complete picture of actions in the system.

The scope of application is all levels of the Code platform — from individual projects and groups to instance configuration.  
Events are recorded regardless of user role (if they have the right to perform the action) and stored centrally.

## Technical capabilities for event logging

The audit system features the following capabilities:

- **Centralized event logging**: All events are stored in a single audit log.
- **Real-time collection**: Events are logged instantly at the time of business operation execution. This includes, for example, user login, email change, or enabling `force push` in a protected branch.
- **Integrity protection**: The audit log is read-only for administrators. Logged events can't be deleted or modified.
- **Access via UI or API**: Events can be viewed and filtered both in the [administrator interface](#accessing-via-ui) and through the [dedicated API](#accessing-via-api).

## Use cases

Audit events let you track:

- Who and when changed a user's access level in a Code project.
- Cases of user creation and deletion.
- Traces of suspicious activity — for example, cases of a bulk employee email change or a repository deletion.
- Changes made to CI/CD environment variables.
- Changes to project and group visibility levels.

Audit events help you:

- Assess risks and strengthen security measures.
- Respond promptly to incidents.

## Accessing audit events

### Accessing via UI

To access audit events, switch to the admin mode and select "Audit events" in the sidebar. This opens the audit event table.

Description of the table columns:

- **Author**: User who triggered the event.
- **Event**: System message with event details.
- **Object**: Related scope of the event (instance, user, group, or project name).
- **Target**: Entity that was changed (project, user, protected branch, token, or CI variable name, etc.).
- **Event time**: Date and time when the event occurred.

![Audit event table](/images/code/audit_events_table_en.png)

### Accessing via API

Deckhouse Code provides the following API method to retrieve the list of audit events:

`POST /api/v4/admin/audit_events/search`

The method supports filtering by dates, full-text search, and entity types.

{% alert level="warning" %}
The date range must be within a single calendar month. If `created_after` and `created_before` belong to different months, the `created_before` value will be automatically adjusted to the last day of the month of `created_after`.
{% endalert %}

#### Request parameters

| Parameter        | Type   | Required | Description                                                                                  |
|------------------|--------|----------|----------------------------------------------------------------------------------------------|
| `created_after`  | String | No       | Start date (inclusive) in ISO8601 format. Default: beginning of the current month.           |
| `created_before` | String | No       | End date (inclusive) in ISO8601 format. Default: end of the current month.                   |
| `q`              | String | No       | Full-text search in the event message.                                                       |
| `sort`           | String | No       | Sort by creation date. Possible values: `created_asc`, `created_desc` (default).             |
| `entity_types`   | Array  | No       | List of entity types for filtering: `User`, `Project`, `Group`, `Gitlab::Audit::InstanceScope`. |

Example request:

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

## Audit event contents

Each audit event contains a date, time, IP address and user account details, as well as all required information about the scope of changes, object, and what exactly has been changed.

## List of audit events

The table below shows example system messages.  
Audit events in a production environment contain full information either directly in the message or in an additional JSON field with data.  

<div class="table-wrapper" markdown="0">
<table class="supported_versions" markdown="0" style="table-layout: fixed">
<thead>
<tr>
<th>Name</th>
<th>System message</th>
<th>Purpose</th>
<th>Audited attributes</th>
</tr>
</thead>
<tbody>
<tr>
<td><code style="word-break: break-all; white-space: normal;">2fa_login_failed</code></td>
<td>User 2fa login failed</td>
<td>A failed attempt to log in with two-factor authentication was detected.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_approved</code></td>
<td>User access was approved</td>
<td>User's access request to the instance was approved.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_token_created</code></td>
<td>Project/Group access token created</td>
<td>An access token for a project or group was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">access_token_revoked</code></td>
<td>Project/Group access token revoked</td>
<td>An access token was revoked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">added_gpg_key</code></td>
<td>Added new gpg key to user</td>
<td>A user added a new GPG key.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">added_ssh_key</code></td>
<td>User added new ssh key</td>
<td>A user added a new SSH key.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_created</code></td>
<td>Application was created</td>
<td>A new application (OAuth or integration) was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_deleted</code></td>
<td>Application deleted</td>
<td>An application was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_secret_renew</code></td>
<td>Application secret renew</td>
<td>The application secret was renewed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">application_updated</code></td>
<td>Application Updated</td>
<td>Application parameters were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_cd_job_token_removed_from_allowlist</code></td>
<td>Disallow group to use job token</td>
<td>A project restricted a specific group from using CI/CD Job Token.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_cd_job_token_added_to_allowlist</code></td>
<td>Allow group to use job token</td>
<td>A project allowed a specific group to use CI/CD Job Token.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_created</code></td>
<td>Ci variable <code>#{key}</code> created</td>
<td>A new CI/CD variable was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_deleted</code></td>
<td>Ci variable <code>#{key}</code> deleted</td>
<td>A CI/CD variable was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_variable_updated</code></td>
<td>Ci variable updated (Value, Protected)</td>
<td>The value or protection status of a CI/CD variable was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_created</code></td>
<td>Deploy key added</td>
<td>A new deploy key was added for a project or instance.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_deleted</code></td>
<td>Deploy key was deleted</td>
<td>A deploy key was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_disabled</code></td>
<td>Deploy key disabled</td>
<td>A deploy key was disabled.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_key_enabled</code></td>
<td>Deploy key enabled</td>
<td>A deploy key was re-enabled.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_created</code></td>
<td>Deploy token created</td>
<td>A deploy token was created for data access.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_deleted</code></td>
<td>Deploy token deleted</td>
<td>A deploy token was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">deploy_token_revoked</code></td>
<td>Deploy token revoked</td>
<td>A deploy token was revoked by a user or the system.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_created</code></td>
<td>Created feature flag with description</td>
<td>A new feature flag was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_deleted</code></td>
<td>Feature flag was deleted</td>
<td>A feature flag was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">feature_flag_updated</code></td>
<td>Feature flag was updated</td>
<td>A feature flag was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_created</code></td>
<td>Group was created</td>
<td>A new group was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_export_created</code></td>
<td>Group file export was created</td>
<td>A group export file was generated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_created</code></td>
<td>Invited group to group</td>
<td>Another group was invited to the group using a group link.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_deleted</code></td>
<td>Revoked group from group</td>
<td>Group access through a group link was revoked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_group_link_updated</code></td>
<td>Group access changed</td>
<td>Group access parameters through a group link were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_created</code></td>
<td>Invited group to project</td>
<td>A group was invited to a project using a group link.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_deleted</code></td>
<td>Revoked group from project</td>
<td>Group access to a project was revoked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_invite_via_project_group_link_updated</code></td>
<td>Group access for project changed</td>
<td>Group access parameters to a project were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_updated</code></td>
<td>Group updated (visibility, 2FA grace period)</td>
<td>Changes in group settings (visibility, security, limits, access policy).</td>
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
<td>An administrator started a session impersonating another user.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">impersonation_stopped</code></td>
<td>User root stopped impersonation</td>
<td>An administrator stopped impersonating another user.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">instance_settings_updated</code></td>
<td>Instance settings updated: Signup enabled turned on</td>
<td>Global instance settings were updated.</td>
<td>All instance settings except encrypted fields.</td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">login_failed</code></td>
<td>Attempt to login failed</td>
<td>A login attempt failed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">manually_trigger_housekeeping</code></td>
<td>Housekeeping task</td>
<td>A housekeeping task for a repository was triggered manually.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_created</code></td>
<td>New member access granted</td>
<td>A user was granted membership (role) in a group or project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_destroyed</code></td>
<td>Member access revoked</td>
<td>A user's membership was revoked from a group or project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">member_permissions_updated</code></td>
<td>Member access updated</td>
<td>A user's role or membership expiration date was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_closed_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> closed by project bot</td>
<td>A merge request was closed by a project bot.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_created_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> created by project bot</td>
<td>A merge request was created by a project bot.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_merged_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> merged by project bot</td>
<td>A merge request was merged by a project bot.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">merge_request_reopened_by_project_bot</code></td>
<td>Merge request <code>#{merge_request.title}</code> reopened by project bot</td>
<td>A merge request was reopened by a project bot.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">omniauth_login_failed</code></td>
<td>Omniauth login failed for <code>#{user}</code> <code>#{provider}</code></td>
<td>Failed login attempt via external OAuth/Omniauth provider.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">password_reset_failed</code></td>
<td>Password reset failed</td>
<td>Failed password reset attempt by a user.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">personal_access_token_issued</code></td>
<td>Personal access token issued</td>
<td>A new personal access token was issued.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">personal_access_token_revoked</code></td>
<td>Personal access token revoked</td>
<td>A personal access token was revoked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">pipeline_deleted</code></td>
<td>Pipeline deleted</td>
<td>A CI/CD pipeline was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_blobs_removal</code></td>
<td>Project blobs removed</td>
<td>Bulk removal of repository blobs in a project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_created</code></td>
<td>Project was created</td>
<td>A new project was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_default_branch_changed</code></td>
<td>Project default branch updated</td>
<td>The default branch of a project was changed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_export_created</code></td>
<td>Project export created</td>
<td>A project export file was generated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_feature_updated</code></td>
<td>Project features updated</td>
<td>Feature access levels for a project were updated (issues, wiki, etc.).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_setting_updated</code></td>
<td>Project settings updated</td>
<td>Project merge commit and squash commit templates were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_text_replacement</code></td>
<td>Project text replaced</td>
<td>Bulk text replacement in a project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_topic_changed</code></td>
<td>Project topic changed</td>
<td>Project topic was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_updated</code></td>
<td>Project updated (name, namespace)</td>
<td>Project settings were updated (name, namespace, policies).</td>
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
<td>A protected branch was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_branch_deleted</code></td>
<td>Protected branch was deleted</td>
<td>A protected branch was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_branch_updated</code></td>
<td>Protected branch was updated</td>
<td>Protected branch rules were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_created</code></td>
<td>Protected tag created</td>
<td>A protected tag was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_deleted</code></td>
<td>Protected tag was deleted</td>
<td>A protected tag was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">protected_tag_updated</code></td>
<td>Protected tag updated</td>
<td>Protected tag rules were updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">removed_gpg_key</code></td>
<td>Removed gpg key from user</td>
<td>A user's GPG key was removed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">removed_ssh_key</code></td>
<td>User removed ssh key</td>
<td>A user's SSH key was removed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">requested_password_reset</code></td>
<td>User requested password change</td>
<td>A user requested a password reset.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">revoked_gpg_key</code></td>
<td>Revoked gpg key from user</td>
<td>A user's GPG key was revoked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unban_user</code></td>
<td>User was unban</td>
<td>A user was unbanned.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unblock_user</code></td>
<td>User was unblocked</td>
<td>A user was unblocked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_access_locked</code></td>
<td>User access locked</td>
<td>A user account was locked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_access_unlocked</code></td>
<td>User access unlocked</td>
<td>A user account was unlocked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_activated</code></td>
<td>User was activated</td>
<td>A user account was activated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_banned</code></td>
<td>User was banned</td>
<td>A user account was banned.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_blocked</code></td>
<td>User was blocked</td>
<td>A user account was blocked.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_created</code></td>
<td>User was created</td>
<td>A new user account was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_deactivated</code></td>
<td>User was deactivated</td>
<td>A user account was deactivated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_destroyed</code></td>
<td>User was destroyed</td>
<td>A user account was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_email_updated</code></td>
<td>User email updated</td>
<td>A user's email address was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_logged_in</code></td>
<td>User logged in</td>
<td>A user logged in successfully.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_password_updated</code></td>
<td>Password updated</td>
<td>A user's password was changed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_rejected</code></td>
<td>User was rejected</td>
<td>A user account was rejected (for example, during a registration).</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_removed_two_factor</code></td>
<td>Two factor disabled</td>
<td>A user disabled two-factor authentication.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_settings_updated</code></td>
<td>User settings updated</td>
<td>A user updated their profile settings.</td>
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
<td>A new user registered.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_switched_to_admin_mode</code></td>
<td>User switched to admin mode</td>
<td>A user switched to admin mode.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">user_username_updated</code></td>
<td>Username updated</td>
<td>A user's username was updated.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">webhook_created</code></td>
<td>Webhook was created</td>
<td>A webhook for a project, group, or instance was created.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">webhook_destroyed</code></td>
<td>System hook removed</td>
<td>A webhook was removed.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">group_deleted</code></td>
<td>Group was deleted</td>
<td>A group was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">project_deleted</code></td>
<td>Project was deleted</td>
<td>A project was deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">logout</code></td>
<td>User logged out</td>
<td>A user logged out of the system.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">unauthenticated_session</code></td>
<td>Redirected to login</td>
<td>An unauthenticated user was redirected to the login page.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runners_bulk_deleted</code></td>
<td>CI runner bulk deleted: Errors:</td>
<td>Multiple CI runners were deleted.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_registered</code></td>
<td>CI runner created via API</td>
<td>A CI runner was registered via API.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_unregistered</code></td>
<td>CI runner unregistered</td>
<td>A CI runner was unregistered.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_token_reset</code></td>
<td>CI runner registration token reset</td>
<td>A CI runner's registration token was reset.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_assigned_to_project</code></td>
<td>CI runner assigned to project</td>
<td>A CI runner was assigned to a project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_unassigned_from_project</code></td>
<td>CI runner unassigned from project</td>
<td>A CI runner was unassigned from a project.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">ci_runner_created</code></td>
<td>CI runner created via UI</td>
<td>A CI runner was created via the user interface.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">package_registry_package_published</code></td>
<td><code style="word-break: break-all; white-space: normal;">#{name}</code> package version <code>#{version}</code> has been published</td>
<td>A new package was published to the package registry.</td>
<td></td>
</tr>
<tr>
<td><code style="word-break: break-all; white-space: normal;">package_registry_package_deleted</code></td>
<td>package version <code>#{package.version}</code> has been deleted</td>
<td>A package was deleted from the package registry.</td>
<td></td>
</tr>
</tbody>
</table>
</div>
