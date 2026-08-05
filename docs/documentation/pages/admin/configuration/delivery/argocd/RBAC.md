---
title: "Configuring role-based access control"
permalink: en/admin/configuration/delivery/argocd/rbac/
description: "Configuring Argo CD role-based access control in Deckhouse Kubernetes Platform."
lang: en
relatedLinks:
  - title: "Official Argo CD website"
    url: "https://argo-cd.readthedocs.io"
  - title: "Official Argo CD Operator website"
    url: "https://argocd-operator.readthedocs.io"
---

Argo CD uses its own role-based access control (RBAC) model, which is not based on the Kubernetes or Deckhouse Kubernetes Platform role model. The Argo CD role model lets you restrict access to resources and operations through its own policies and roles.

Before configuring role-based access control, complete [authentication and authorization setup](../authentication/). After that, assign roles to users and groups and set permissions at the level of the entire Argo CD instance or individual projects with an [AppProject](/modules/operator-argo/cr.html#appproject) object.

You can define role-based access control (RBAC) rules in two places:

- globally — in the [ArgoCD](/modules/operator-argo/cr.html#argocd) object;
- at the project level — in the roles of an [AppProject](/modules/operator-argo/cr.html#appproject) object.

## Built-in roles

Argo CD has two predefined roles with a set of policies:

- `role:readonly` — read-only access;
- `role:admin` — access with administrator privileges.

{% offtopic title="Full description of predefined role policies..." %}

```text
# Policies of the readonly role.
p, role:readonly, applications, get, */*, allow
p, role:readonly, applicationsets, get, */*, allow
p, role:readonly, certificates, get, *, allow
p, role:readonly, clusters, get, *, allow
p, role:readonly, repositories, get, *, allow
p, role:readonly, write-repositories, get, *, allow
p, role:readonly, projects, get, *, allow
p, role:readonly, accounts, get, *, allow
p, role:readonly, gpgkeys, get, *, allow
p, role:readonly, logs, get, */*, allow

# Policies of the admin role.
p, role:admin, applications, create, */*, allow
p, role:admin, applications, update, */*, allow
p, role:admin, applications, update/*, */*, allow
p, role:admin, applications, delete, */*, allow
p, role:admin, applications, delete/*, */*, allow
p, role:admin, applications, sync, */*, allow
p, role:admin, applications, override, */*, allow
p, role:admin, applications, action/*, */*, allow
p, role:admin, applicationsets, get, */*, allow
p, role:admin, applicationsets, create, */*, allow
p, role:admin, applicationsets, update, */*, allow
p, role:admin, applicationsets, delete, */*, allow
p, role:admin, certificates, create, *, allow
p, role:admin, certificates, update, *, allow
p, role:admin, certificates, delete, *, allow
p, role:admin, clusters, create, *, allow
p, role:admin, clusters, update, *, allow
p, role:admin, clusters, delete, *, allow
p, role:admin, repositories, create, *, allow
p, role:admin, repositories, update, *, allow
p, role:admin, repositories, delete, *, allow
p, role:admin, write-repositories, create, *, allow
p, role:admin, write-repositories, update, *, allow
p, role:admin, write-repositories, delete, *, allow
p, role:admin, projects, create, *, allow
p, role:admin, projects, update, *, allow
p, role:admin, projects, delete, *, allow
p, role:admin, accounts, update, *, allow
p, role:admin, gpgkeys, create, *, allow
p, role:admin, gpgkeys, delete, *, allow
p, role:admin, exec, create, */*, allow

# The admin role includes all privileges of the readonly role.
g, role:admin, role:readonly

# The local admin user is assigned the admin role.
g, admin, role:admin
```

{% endofftopic %}

## Default policy for authenticated users

After successful authentication, the user receives the role specified in the [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) parameter of ArgoCD.

{% alert level="warning" %}
All authenticated users receive at least the permissions defined in the [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) parameter. These rights cannot be revoked with a `deny` effect rule.
{% endalert %}

Create a separate role, for example `role:authenticated`, grant it a minimal set of permissions, and use it in [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy).

Example:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  rbac:
    defaultPolicy: role:authenticated
    policy: |
      p, role:authenticated, applications, get, */*, allow
...
```

## Anonymous access

If you enable anonymous access to an Argo CD instance, users can receive rights according to the policy specified in the [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) parameter without authentication.

Anonymous access is enabled with the [`spec.usersAnonymousEnabled`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-usersanonymousenabled) parameter of ArgoCD.

{% alert level="warning" %}
When enabling anonymous access, create a separate default role, for example `role:unauthenticated`, and assign it in [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy).
{% endalert %}

## Role-based access control structure

The Argo CD role-based access control syntax is based on the [Casbin](https://casbin.org/docs/overview) model and uses two types of records:

- binding a user or group to a role;
- assigning permissions to a role, user, or group.

### Role binding

Syntax:

```text
g, <user/group>, <role>
```

Where:

- `<user/group>` — a local user, SSO user, or group;
- `<role>` — an internal Argo CD role.

Example:

```text
g, my-org:team-beta, role:admin
g, user@example.org, role:readonly
```

### Access policy

Syntax:

```text
p, <role/user/group>, <resource>, <action>, <object>, <effect>
```

Where:

- `<role/user/group>` — the subject to which the rule is assigned;
- `<resource>` — the resource type;
- `<action>` — the allowed operation;
- `<object>` — the object to which the rule applies;
- `<effect>` — the check result: `allow` or `deny`.

Example:

```csv
p, role:developer, applications, sync, dev-project/*, allow
p, role:developer, logs, get, dev-project/*, allow
```

{% alert level="info" %}
To assign rules to a group, first bind the group to a role with a `g, <group>, <role>` record. After that, assign permissions to the role itself.
{% endalert %}

## Resources and supported actions

The following are the main Argo CD resources and actions that you can use in RBAC policies.

| Resource | get | create | update | delete | sync | action | override | invoke |
| :-- | :-: | :--: | :--: | :--: | :--: | :--: | :--: | :--: |
| `applications` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| `applicationsets` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `clusters` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `projects` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `repositories` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `accounts` | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `certificates` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `gpgkeys` | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| `logs` | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `exec` | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `extensions` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |

## Application-bound policies

Some resources are bound to specific applications:

- `applications`;
- `applicationsets`;
- `logs`;
- `exec`.

For such resources, the `<object>` value usually has the `<APP_PROJECT>/<APP_NAME>` format.

Example:

```csv
p, example-user, applications, get, *, allow
p, example-user, logs, get, example-project/my-app, allow
```

If Argo CD has application placement in arbitrary namespaces enabled, the `<object>` format changes to `<APP_PROJECT>/<APP_NAMESPACE>/<APP_NAME>`.

Example:

```csv
p, example-user, applications, get, */app-namespace/*, allow
p, example-user, logs, get, example-project/app-namespace/my-app, allow
```

## The `applications` resource

The `applications` resource supports both basic actions and more detailed rules.

### Detailed permissions for `update` and `delete`

The `update` and `delete` permissions granted for an application allow changing or deleting the application itself, but not its nested resources.

To allow an operation on application resources, use the format:

```text
<action>/<group>/<kind>/<namespace>/<name>
```

For example, to allow a user to delete only Pods in the `prod-app` application:

```csv
p, example-user, applications, delete/*/Pod/*/*, default/prod-app, allow
```

To allow updating all application resources but not the application itself:

```csv
p, example-user, applications, update/*, default/prod-app, allow
```

To deny deleting the application but allow deleting Pods:

```csv
p, example-user, applications, delete, default/prod-app, deny
p, example-user, applications, delete/*/Pod/*/*, default/prod-app, allow
```

To allow updating the application but deny updating any subresources:

```csv
p, example-user, applications, update, default/prod-app, allow
p, example-user, applications, update/*, default/prod-app, deny
```

{% alert level="warning" %}
Argo CD does not use the `/` character as a separator when comparing glob patterns. Therefore, always specify the full resource path, that is, use patterns with four `/` characters.
{% endalert %}

### The `action` action

The `action` action corresponds to built-in or custom actions on application resources.

Format of the `<action>` value:

```text
action/<group>/<kind>/<action-name>
```

For example:

- `action/extensions/DaemonSet/restart` — an action for `DaemonSet`;
- `action//Pod/maintenance-off` — an action for `Pod` that has no API group.

Policy example:

```csv
p, example-user, applications, action//Pod/maintenance-off, default/*, allow
p, example-user, applications, action/extensions/DaemonSet/*, default/*, allow
```

To allow all actions on application resources:

```csv
p, example-user, applications, action/*, default/*, allow
```

### The `override` action

The `override` permission allows passing arbitrary manifests or another revision when synchronizing an `Application`.

{% alert level="warning" %}
The `override` right lets a user effectively change the composition or state of deployed application resources. Grant it only to users who truly need it.
{% endalert %}

If the `application.sync.requireOverridePrivilegeForRevisionSync: 'true'` parameter is enabled in the `argocd-cm` ConfigMap, passing an arbitrary revision during synchronization also requires the `override` right.

## Other resources

### The `applicationsets` resource

The `applicationsets` resource also belongs to application-bound policies. The `create` permission for this resource effectively allows creating [Application](/modules/operator-argo/cr.html#application) objects through an [ApplicationSet](/modules/operator-argo/cr.html#applicationset) object.

Example:

```csv
p, dev-group, applicationsets, *, dev-project/*, allow
```

With this rule, users from `dev-group` can create an ApplicationSet that manages applications only within the `dev-project` project.

### The `logs` resource

The `get` permission for the `logs` resource allows viewing Pod logs of an application through the Argo CD web interface. In meaning, this is equivalent to `d8 k logs`.

### The `exec` resource

The `create` permission for the `exec` resource allows opening an `exec` session in an application Pod through the Argo CD web interface. In meaning, this is similar to `d8 k exec`.

### The `extensions` resource

The `extensions` resource is used to invoke proxy extensions. Checks for such rules work together with the `applications` resource: the user must have read access to the application from which the request is made.

Example:

```csv
p, example-user, applications, get, default/*, allow
p, example-user, extensions, invoke, httpbin, allow
```

### The `deny` effect

If a policy with the `deny` effect matches a request, access is denied even if there are more specific rules with the `allow` effect.

The order of lines in [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) does not affect the result.

## Policy evaluation order and matching modes

Access checks are performed in two stages:

1. Rules from [`spec.rbac.defaultPolicy`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-defaultpolicy) are checked.
1. If no decision is found, rules related to the user and their groups are checked.

If access is explicitly allowed or denied by the default policy, further checks are not performed.

Argo CD supports two matching modes for values set in [`spec.rbac.policyMatchMode`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-rbac-policymatchmode):

- `glob` — matching by glob patterns;
- `regex` — matching by regular expressions.

### Matching in `glob` mode

In `glob` mode, tokens are treated as whole strings without special separators.

Policy example:

```csv
p, example-user, applications, action/extensions/*, default/*, allow
```

If the `example-user` user invokes the `extensions/DaemonSet/test` action, the following checks are performed:

1. `example-user` matches `example-user`.
1. `applications` matches `applications`.
1. `action/extensions/DaemonSet/test` matches `action/extensions/*`.
1. `default/my-app` matches `default/*`.

## Using SSO users and groups

The `scopes` parameter defines which OIDC scopes Argo CD must analyze during RBAC checks in addition to `sub`. If the parameter is not set, the default value is `'[groups]'`.

Example of an ArgoCD object with scopes configured:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  rbac:
    policy: |
      p, my-org:team-alpha, applications, sync, my-project/*, allow
      g, my-org:team-beta, role:admin
      g, user@example.org, role:admin
      g, admin, role:admin
      g, role:admin, role:readonly
    defaultPolicy: role:readonly
    scopes: '[groups, email]'
```

In this example:

1. `g, admin, role:admin` explicitly binds the built-in `admin` user to the `role:admin` role.
1. `g, role:admin, role:readonly` sets role inheritance: everyone who has `role:admin` automatically also gets `role:readonly` rights.

You can combine this approach with roles at the [AppProject](/modules/operator-argo/cr.html#appproject) level. For example, you can create the `team-beta-project` project and assign rights to users and groups:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: team-beta-project
  namespace: argocd
spec:
  roles:
    - name: admin
      description: Administrator permissions for team-beta.
      policies:
        - p, proj:team-beta-project:admin, applications, *, team-beta-project/*, allow
      groups:
        - user@example.org
        - my-org:team-beta
```

## Local users

You can grant access to local users in two ways:

- assign rules directly;
- bind the user to a role.

Example of assigning a rule directly:

```csv
p, my-local-user, applications, sync, my-project/*, allow
```

Example of binding to a role:

```csv
g, my-local-user, role:admin
```

{% alert level="warning" %}
If SSO and local users are used at the same time, ambiguity is possible. For example, if the local user `sally` is bound to `role:admin`, and one of the SSO scope values is also `sally`, that SSO user may also receive `role:admin` rights.
{% endalert %}

To avoid this situation, when using SSO and local users together, assign rules to local users directly rather than through roles.

Example:

```csv
p, my-local-user, *, *, *, allow
```

## Testing RBAC policies

To verify RBAC rules, use the `argocd` CLI utility. To check whether a specific user, group, or role can perform a certain action, use the following command:

```bash
argocd admin settings rbac can
```

For example, run the command below to check whether the `admin@deckhouse.io` user has rights to create `applications` in the `default` project:

```bash
argocd admin settings rbac can admin@deckhouse.io create application 'default/*' --namespace argocd
```
