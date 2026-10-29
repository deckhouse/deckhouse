---
title: Шаблоны
permalink: ru/architecture/marketplace/templates.html
description: "Helm-шаблоны пакета Application: значения шаблонов, именование объектов, образы контейнеров и доступ к хранилищу образов, Ingress и адреса приложения, учётные данные в веб-интерфейсе, метки и состояние рабочих нагрузок."
lang: ru
search: application templates, d8a prefix, Application.Instance, Platform.applications, application-endpoint-description, application-ingress, application-instance, initial-secret, showed-credentials, шаблоны приложения, имена объектов, образы пакета, учётные данные приложения
---

{% raw %}

Каталог `templates/` пакета содержит Helm-шаблоны. Deckhouse Platform (DP) обрабатывает их и применяет результат с помощью Nelm, поэтому в шаблонах можно использовать [аннотации Nelm](nelm-annotations.html) для управления порядком развёртывания, жизненным циклом ресурсов и отслеживанием готовности. На этой странице описаны данные, которые DP передаёт в шаблоны, и правила, которым шаблоны должны следовать.

## Значения шаблонов

DP обрабатывает шаблоны с тремя корнями значений:

| Корень | Содержимое |
|---|---|
| `.Values` | Values приложения: настройки со значениями по умолчанию, внутренние значения и значения, заданные хуками (подробнее — в подразделе [«Как DP формирует values»](settings.html#как-dp-формирует-values)). Значения не вложены в ключ с именем пакета: настройка доступна как `.Values.<SETTING_NAME>` |
| `.Application` | Параметры экземпляра и пакета |
| `.Platform` | Глобальные параметры DP (только для чтения) |

### Корень .Application

Корень `.Application` содержит параметры экземпляра и пакета.

| Путь | Описание |
|---|---|
| `.Application.Instance.Name` | Имя экземпляра, то есть имя ресурса Application |
| `.Application.Instance.Namespace` | Неймспейс, в который установлено приложение |
| `.Application.Package.Name` | Имя пакета (поле `name` в `package.yaml`) |
| `.Application.Package.Version` | Версия пакета |
| `.Application.Package.Images` | Образы контейнеров пакета: словарь, сопоставляющий имена образов со ссылками на образы с дайджестом (подробнее — [в подразделе «Образы контейнеров»](#образы-контейнеров)) |
| `.Application.Package.Registry` | Параметры доступа к репозиторию, из которого установлен пакет (подробнее — [в подразделе «Образы контейнеров»](#образы-контейнеров)) |
| `.Application.Settings` | Итоговые настройки: `Application.spec.settings` с подставленными значениями по умолчанию |

### Корень .Platform

Корень `.Platform` содержит глобальные параметры DP. В шаблонах приложений полезны следующие параметры:

| Путь | Описание |
|---|---|
| `.Platform.applications.publicDomainTemplate` | Шаблон DNS-имён для Ingress приложений (подробнее — [в подразделе «Ingress и HTTPS»](#ingress-и-https)) |
| `.Platform.applications.ingressClass` | IngressClass для Ingress приложений. По умолчанию: `nginx` |
| `.Platform.applications.https.mode` | Режим HTTPS для приложений: `CertManager` (по умолчанию), `CustomCertificate`, `Disabled` или `OnlyInURI` |
| `.Platform.applications.https.certManager.clusterIssuerName` | ClusterIssuer [модуля `cert-manager`](/modules/cert-manager/) для сертификатов приложений. По умолчанию: `letsencrypt` |
| `.Platform.discovery.clusterDomain` | Домен кластера, например, `cluster.local` |
| `.Platform.discovery.kubernetesVersion` | Версия Kubernetes в кластере |
| `.Platform.deckhouseVersion` | Версия DP |
| `.Platform.deckhouseEdition` | Редакция DP |

Параметры `.Platform.applications` администратор задаёт для всех приложений кластера [в разделе `applications`](../../reference/api/global.html#parameters-applications) глобальных настроек.

## Имена объектов

Называйте каждый объект в шаблонах в формате `d8a-<INSTANCE_NAME>-<SUFFIX>`:

```yaml
metadata:
  name: d8a-{{ .Application.Instance.Name }}-server
```

Команда `d8 package verify` сообщает об ошибке (правило `instance-prefix`) для каждого отрендеренного объекта, имя которого не начинается с `d8a-{{ .Application.Instance.Name }}-`. Для объектов Job и CronJob суффикс после этого префикса должен быть не длиннее 23 символов (правило `job-name`).

Префикс нужен для следующего:

- Несколько экземпляров пакета можно установить в один неймспейс без конфликтов имён.
- Префикс `d8a-` зарезервирован для приложений: admission-политика `d8a-prefix.deckhouse.io` запрещает пользователям создавать, изменять и удалять объекты с именами, начинающимися с `d8a-`. Разрешено удалять поды и изменять Secret с лейблом `packages.deckhouse.io/initial-secret` (подробнее — [в разделе «Учётные данные в веб-интерфейсе»](#учётные-данные-в-веб-интерфейсе)).
- DP добавляет тот же префикс к именам объектов, которые хуки создают, изменяют или удаляют. Поэтому хук может обращаться к объекту, отрендеренному из шаблонов, только по суффиксу (подробнее — [в разделе «Хуки»](hooks.html#имена-объектов)).

Не используйте `.Release.Name` в именах объектов: Helm-релиз приложения называется `<NAMESPACE>.<INSTANCE_NAME>`, а точка недопустима в именах ресурсов многих типов.

Используйте короткие суффиксы: имя экземпляра может быть длиной до 24 символов, а полное имя объекта должно укладываться в ограничения Kubernetes (подробнее — [в подразделе «Ограничения на имена»](concepts.html#ограничения-на-имена)).

## Лейблы и защита объектов

DP добавляет следующие лейблы в метаданные каждого отрендеренного объекта:

| Лейбл | Значение |
|---|---|
| `heritage` | `deckhouse` |
| `packages.deckhouse.io/package` | Имя пакета |
| `packages.deckhouse.io/instance` | Имя экземпляра |
| `health.deckhouse.io/package` | `<NAMESPACE>.<INSTANCE_NAME>` |

Лейблы добавляются только в метаданные отрендеренных объектов, но не в шаблоны подов. Лейблы, которые используются в селекторах подов, задавайте в шаблонах самостоятельно.

При удалении Application DP находит объекты, оставленные экземпляром в неймспейсе, по лейблу `packages.deckhouse.io/instance` (подробнее — [в подразделе «Orphan-ресурсы»](lifecycle.html#orphan-ресурсы)).

Объекты с лейблом `heritage: deckhouse` защищает admission-политика `label-objects.deckhouse.io`, а объекты с префиксом имени `d8a-` — политика `d8a-prefix.deckhouse.io`. В результате:

- Пользователи не могут изменять или удалять объекты, отрендеренные из шаблонов. Исключение — Secret с лейблом `packages.deckhouse.io/initial-secret`: пользователи могут их изменять, но не создавать и не удалять (подробнее — [в разделе «Учётные данные в веб-интерфейсе»](#учётные-данные-в-веб-интерфейсе)). Чтобы изменить их вручную, например, при отладке, переведите приложение в [режим обслуживания](lifecycle.html#режим-обслуживания).
- Рабочие нагрузки приложения тоже не могут изменять объекты, отрендеренные из шаблонов.
- Политика `d8a-prefix.deckhouse.io` не действует на сервисные аккаунты, имена которых начинаются с `d8a-`. Рабочие нагрузки, работающие от имени таких сервисных аккаунтов, могут управлять созданными ими объектами с префиксом `d8a-`, а эти объекты остаются защищёнными от пользователей.

## Образы контейнеров

Ссылайтесь на образы контейнеров пакета через `.Application.Package.Images`. Ключ — имя каталога образа в `images/`, преобразованное в camelCase (например, `images/my-server` становится `myServer`). Значение — полная ссылка на образ с дайджестом, например, `registry.example.com/packages/myapp@sha256:...`:

```yaml
containers:
  - name: server
    image: {{ index .Application.Package.Images "server" }}
```

Команда `d8 package render` использует имена каталогов как есть, поэтому называйте каталоги образов одним словом: тогда один и тот же ключ работает и при локальном рендеринге, и в кластере.

Чтобы загружать образы из репозитория, создайте Secret для загрузки образов из `.Application.Package.Registry.dockercfg` и укажите его в `imagePullSecrets`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-registrysecret
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ .Application.Package.Registry.dockercfg }}
```

Раздел `.Application.Package.Registry` содержит следующие поля:

| Поле | Описание |
|---|---|
| `name` | Имя ресурса PackageRepository |
| `repository` | Адрес репозитория из ресурса PackageRepository |
| `dockercfg` | Конфигурация Docker для доступа к репозиторию в кодировке Base64. Если в PackageRepository заданы `login` и `password`, DP формирует конфигурацию из них |
| `scheme` | Протокол доступа к репозиторию в том виде, в котором он указан в ресурсе PackageRepository, например, `HTTPS`. DP передаёт значение как есть: оно может быть в любом регистре или пустым, что означает HTTPS |
| `ca` | CA-сертификат репозитория, если он задан |

## Ingress и HTTPS

Формируйте имена хостов и параметры TLS для Ingress приложения из параметров `.Platform.applications`:

- `publicDomainTemplate` — шаблон DNS-имени с двумя подстановками `%s`. Первая заменяется именем экземпляра, вторая — неймспейсом. Например, с шаблоном `%s.%s.apps.example.com` экземпляр `grafana` в неймспейсе `monitoring` получает имя хоста `grafana.monitoring.apps.example.com`. Если параметр не задан, не создавайте Ingress;
- `ingressClass` — IngressClass для Ingress;
- `https.mode` — режим HTTPS. В режиме `CertManager` запрашивайте сертификат у ClusterIssuer, указанного в `https.certManager.clusterIssuerName`. В режимах `Disabled` и `OnlyInURI` не настраивайте TLS на Ingress.

Пример Ingress:

```yaml
{{- $applications := .Platform.applications }}
{{- if $applications.publicDomainTemplate }}
{{- $host := printf $applications.publicDomainTemplate .Application.Instance.Name .Application.Instance.Namespace }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Web UI"
    {{- if eq $applications.https.mode "CertManager" }}
    cert-manager.io/cluster-issuer: {{ $applications.https.certManager.clusterIssuerName }}
    {{- end }}
spec:
  ingressClassName: {{ $applications.ingressClass }}
  {{- if eq $applications.https.mode "CertManager" }}
  tls:
    - hosts:
        - {{ $host }}
      secretName: d8a-{{ .Application.Instance.Name }}-web-tls
  {{- end }}
  rules:
    - host: {{ $host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: d8a-{{ .Application.Instance.Name }}-server
                port:
                  name: http
{{- end }}
```

Заготовка пакета, созданная командой `d8 package bootstrap app`, содержит вспомогательные шаблоны для этих параметров в `templates/_helpers/`:

- `public_domain`;
- `ingress_class`;
- `https_mode`;
- `https_ingress_tls_enabled`;
- `https_cert_manager_cluster_issuer_name`.

## Адреса приложения

Чтобы адреса Ingress отображались [в поле `status.urls`](../../reference/api/cr.html#application-v1alpha1-status-urls) ресурса Application (например, веб-интерфейс показывает их как ссылки на приложение), добавьте к Ingress аннотацию `packages.deckhouse.io/application-endpoint-description`:

- Значение аннотации — описание адреса. Значение `"true"` добавляет адрес без описания, а значение `"false"` исключает Ingress.
- DP формирует URL-адрес для каждой пары `host` и `path` из `spec.rules`. Схема — `https`, если хост указан в `spec.tls`, иначе `http`. Правило без путей даёт URL-адрес `<SCHEME>://<HOST>/`. Правила без `host` пропускаются.
- Учитываются только Ingress группы API `networking.k8s.io`, отрендеренные из шаблонов.
- DP обновляет `status.urls` после каждого успешного применения шаблонов.

Пример статуса Application с Ingress из предыдущего раздела:

```yaml
status:
  urls:
    - url: https://myapp.my-namespace.apps.example.com/
      description: Web UI
```

## Учётные данные в веб-интерфейсе

Веб-интерфейс может показывать рядом с адресом приложения учётные данные для входа, например логин и пароль учётной записи администратора, которую приложение создаёт при установке. Для этого добавьте в шаблоны Secret с логином и паролем и привяжите этот Secret к адресу:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web-credentials
  labels:
    packages.deckhouse.io/application-instance: {{ .Application.Instance.Name }}
    packages.deckhouse.io/application-ingress: d8a-{{ .Application.Instance.Name }}-web
    packages.deckhouse.io/initial-secret: "true"
type: Opaque
data:
  login: {{ "admin" | b64enc }}
  password: {{ .Values.internal.adminPassword | b64enc }}
```

Веб-интерфейс находит учётные данные следующим образом:

- Лейбл `packages.deckhouse.io/application-instance` с именем экземпляра обязателен. Веб-интерфейс ищет учётные данные только среди Secret с этим лейблом в неймспейсе приложения. DP не добавляет этот лейбл к объектам, а лейбл `packages.deckhouse.io/instance`, который DP добавляет (подробнее — [в разделе «Лейблы и защита объектов»](#лейблы-и-защита-объектов)), не учитывается.
- Лейбл `packages.deckhouse.io/application-ingress` привязывает Secret к Ingress из шаблонов по имени. Веб-интерфейс показывает учётные данные рядом с адресами, у которых описание этого Ingress. Значение лейбла не может быть длиннее 63 символов, поэтому имя Ingress должно укладываться в это ограничение.
- Вместо лейбла можно указать описание адреса в аннотации `packages.deckhouse.io/application-endpoint-description` у Secret. Если заданы и лейбл, и аннотация, используется описание Ingress.
- Веб-интерфейс показывает учётные данные, только если у приложения есть адреса с подходящим описанием. К адресам без описания учётные данные не привязываются, поэтому указывайте в аннотации Ingress описание, а не `"true"`.
- Лейбл `packages.deckhouse.io/initial-secret` помечает Secret как одноразовые учётные данные. DP создаёт такой Secret один раз и больше его не обновляет: он добавляет к Secret аннотацию `werf.io/resource-policy: skip-update` (уже заданная политика сохраняется, например `skip-delete` превращается в `skip-delete,skip-update`). Admission-политики разрешают пользователям изменять такой Secret, поэтому веб-интерфейс может отметить его как показанный, а пользователи — сменить учётные данные. Без этого лейбла политики запрещают изменение, и учётные данные можно запросить повторно. DP проверяет только наличие лейбла, его значение не важно.

Укажите логин в ключе `login`, а пароль — в ключе `password`. Веб-интерфейс показывает каждый ключ `data` отдельным полем с именем ключа, поэтому не храните в этом Secret другие данные.

У адресов с учётными данными веб-интерфейс показывает кнопку для получения логина и пароля. Получить учётные данные можно только один раз: после показа веб-интерфейс добавляет к Secret аннотацию `packages.deckhouse.io/showed-credentials: "true"` и больше не предлагает этот Secret. Ограничение действует только в веб-интерфейсе: пользователи с правом на чтение Secret в неймспейсе по-прежнему могут его прочитать.

Поскольку Deckhouse Platform не обновляет Secret с лейблом `packages.deckhouse.io/initial-secret`, изменения его шаблона, в том числе `data`, применяются только при создании Secret, например в новом экземпляре.

Сгенерируйте учётные данные один раз и не меняйте их: веб-интерфейс не предлагает Secret повторно, даже если его данные изменились. Не генерируйте пароль в шаблоне: функции вроде `randAlphaNum` дают новое значение при каждом рендеринге (подробнее — [в разделе «Известные проблемы и способы решения»](nelm-annotations.html#известные-проблемы-и-способы-решения)). Вместо этого, например, генерируйте пароль в хуке, если Secret ещё не создан, а иначе берите его из снимка существующего Secret. Значения, которые задают хуки, хранятся только в памяти, поэтому хук должен восстанавливать пароль после перезапуска Deckhouse Platform (подробнее — [в разделе «Values»](hooks.html#values) на странице хуков).

## Ограничения

Шаблоны должны соблюдать [ограничения Application](concepts.html#ограничения-application):

- Создавайте только объекты уровня неймспейса. Не задавайте `metadata.namespace`: DP создаёт объекты в неймспейсе Application, а `d8 package verify` сообщает об ошибке для объектов с этим полем (правило `instance-namespace`).
- Не создавайте объекты CustomResourceDefinition. DP не устанавливает CRD из каталога `crds/` чарта.

По умолчанию `d8 package verify` также требует, чтобы:

- у каждого Deployment и StatefulSet был PodDisruptionBudget, селектор которого соответствует меткам его подов (правило `pdb`);
- у каждого Deployment, StatefulSet и DaemonSet был VerticalPodAutoscaler с политикой для каждого контейнера (правило `vpa`). Тип VerticalPodAutoscaler доступен, только если включён [модуль `vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/), поэтому рендерите его при условии `.Capabilities.APIVersions.Has "autoscaling.k8s.io/v1/VerticalPodAutoscaler"`, как это сделано в заготовке пакета;
- в Service порты контейнеров указывались в `targetPort` по имени (правило `service-port`).

Строгость этих правил можно понизить в `.pkglint.yaml` (подробнее — [в подразделе «Проверка пакета»](application-development.html#проверка-пакета)).

## Состояние рабочих нагрузок

DP определяет, работает ли приложение, только по его Deployment и StatefulSet:

- Условие `Scaled` ресурса Application становится `True`, когда все Deployment и StatefulSet, отрендеренные из шаблонов, завершили развёртывание и имеют нужное количество готовых реплик. DaemonSet, Job, CronJob и поды не учитываются.
- Deployment, развёртывание которого превысило `spec.progressDeadlineSeconds`, переводит `Scaled` в `False` с причиной `Degraded`.
- Первая установка считается завершённой (`Installed=True`) только после применения манифестов и перехода `Scaled` в `True`.

{% endraw %}
{% alert level="warning" %}
[Пакет](../../glossary.html#marketplace-package) должен содержать хотя бы один Deployment или StatefulSet. Иначе условие `Scaled` остаётся в состоянии `Unknown`, и приложение никогда не переходит в состояния `Installed` и `Ready`.
{% endalert %}
