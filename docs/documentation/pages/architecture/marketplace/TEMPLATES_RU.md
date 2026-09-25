---
title: Шаблоны
permalink: ru/architecture/marketplace/templates.html
description: "Helm-шаблоны пакета Application: значения шаблонов, именование объектов, образы контейнеров и доступ к реестру, Ingress и адреса приложения, метки и состояние рабочих нагрузок."
lang: ru
search: application templates, d8a prefix, Application.Instance, Platform.applications, application-endpoint-description, шаблоны приложения, имена объектов, образы пакета
---

{% raw %}

Каталог `templates/` пакета содержит Helm-шаблоны. Deckhouse Platform (DP) рендерит их и применяет результат с помощью Nelm, поэтому в шаблонах можно использовать [аннотации Nelm](nelm-annotations.html) для управления порядком деплоя, жизненным циклом ресурсов и отслеживанием готовности. На этой странице описаны данные, которые DP передаёт в шаблоны, и правила, которым шаблоны должны следовать.

## Значения шаблонов

DP рендерит шаблоны с тремя корнями значений:

| Корень | Содержимое |
|---|---|
| `.Values` | Values приложения: настройки со значениями по умолчанию, внутренние значения и значения, заданные хуками (см. [как DP формирует values](settings.html#как-dp-формирует-values)). Значения не вложены в ключ с именем пакета: настройка доступна как `.Values.<SETTING_NAME>` |
| `.Application` | Параметры экземпляра и пакета |
| `.Platform` | Глобальные параметры DP (только для чтения) |

### .Application

| Путь | Описание |
|---|---|
| `.Application.Instance.Name` | Имя экземпляра, то есть имя ресурса Application |
| `.Application.Instance.Namespace` | Неймспейс, в который установлено приложение |
| `.Application.Package.Name` | Имя пакета (поле `name` в `package.yaml`) |
| `.Application.Package.Version` | Версия пакета |
| `.Application.Package.Images` | Образы контейнеров пакета: словарь, сопоставляющий имена образов со ссылками на образы с дайджестом (см. [«Образы контейнеров»](#образы-контейнеров)) |
| `.Application.Package.Registry` | Параметры доступа к репозиторию, из которого установлен пакет (см. [«Образы контейнеров»](#образы-контейнеров)) |
| `.Application.Settings` | Итоговые настройки: `Application.spec.settings` с подставленными значениями по умолчанию |

### .Platform

`.Platform` содержит глобальные параметры DP. В шаблонах приложений полезны следующие параметры:

| Путь | Описание |
|---|---|
| `.Platform.applications.publicDomainTemplate` | Шаблон DNS-имён для Ingress приложений (см. [«Ingress и HTTPS»](#ingress-и-https)) |
| `.Platform.applications.ingressClass` | IngressClass для Ingress приложений. По умолчанию: `nginx` |
| `.Platform.applications.https.mode` | Режим HTTPS для приложений: `CertManager` (по умолчанию), `CustomCertificate`, `Disabled` или `OnlyInURI` |
| `.Platform.applications.https.certManager.clusterIssuerName` | ClusterIssuer cert-manager для сертификатов приложений. По умолчанию: `letsencrypt` |
| `.Platform.discovery.clusterDomain` | Домен кластера, например, `cluster.local` |
| `.Platform.discovery.kubernetesVersion` | Версия Kubernetes в кластере |
| `.Platform.deckhouseVersion` | Версия DP |
| `.Platform.deckhouseEdition` | Редакция DP |

Параметры `.Platform.applications` администратор задаёт для всех приложений кластера в разделе [`applications`](../../reference/api/global.html#parameters-applications) глобальных настроек.

## Имена объектов

Называйте каждый объект в шаблонах в формате `d8a-<INSTANCE_NAME>-<SUFFIX>`:

```yaml
metadata:
  name: d8a-{{ .Application.Instance.Name }}-server
```

Команда `d8 package verify` сообщает об ошибке (правило `instance-prefix`) для каждого отрендеренного объекта, имя которого не начинается с `d8a-{{ .Application.Instance.Name }}-`. Для объектов Job и CronJob суффикс после этого префикса должен быть не длиннее 23 символов (правило `job-name`).

Префикс нужен для следующего:

- Несколько экземпляров пакета можно установить в один неймспейс без конфликтов имён.
- Префикс `d8a-` зарезервирован для приложений: admission-политика `d8a-prefix.deckhouse.io` запрещает пользователям создавать, изменять и удалять объекты с именами, начинающимися с `d8a-`. Удалять поды разрешено.
- DP добавляет тот же префикс к именам объектов, которые хуки создают, изменяют или удаляют. Поэтому хук может обращаться к объекту, отрендеренному из шаблонов, только по суффиксу (см. [«Хуки»](hooks.html#имена-объектов)).

Не используйте `.Release.Name` в именах объектов: Helm-релиз приложения называется `<NAMESPACE>.<INSTANCE_NAME>`, а точка недопустима в именах ресурсов многих типов.

Используйте короткие суффиксы: имя экземпляра может быть длиной до 24 символов, а полное имя объекта должно укладываться в ограничения Kubernetes (см. [«Ограничения на имена»](concepts.html#ограничения-на-имена)).

## Метки и защита объектов

DP добавляет следующие метки в метаданные каждого отрендеренного объекта:

| Метка | Значение |
|---|---|
| `heritage` | `deckhouse` |
| `packages.deckhouse.io/package` | Имя пакета |
| `packages.deckhouse.io/instance` | Имя экземпляра |
| `health.deckhouse.io/package` | `<NAMESPACE>.<INSTANCE_NAME>` |

Метки добавляются только в метаданные отрендеренных объектов, но не в шаблоны подов. Метки, которые используются в селекторах подов, задавайте в шаблонах самостоятельно.

Объекты с меткой `heritage: deckhouse` защищает admission-политика `label-objects.deckhouse.io`, а объекты с префиксом имени `d8a-` — политика `d8a-prefix.deckhouse.io`. В результате:

- Пользователи не могут изменять или удалять объекты, отрендеренные из шаблонов. Чтобы изменить их вручную, например, при отладке, переведите приложение в [режим обслуживания](lifecycle.html#режим-обслуживания).
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

`.Application.Package.Registry` содержит следующие поля:

| Поле | Описание |
|---|---|
| `repository` | Адрес репозитория из ресурса PackageRepository |
| `dockercfg` | Конфигурация Docker для доступа к репозиторию в кодировке Base64. Если в PackageRepository заданы `login` и `password`, DP формирует конфигурацию из них |
| `scheme` | Протокол доступа к репозиторию: `HTTP` или `HTTPS` |
| `ca` | CA-сертификат репозитория, если он задан |

## Ingress и HTTPS

Формируйте имена хостов и параметры TLS для Ingress приложения из параметров `.Platform.applications`:

- `publicDomainTemplate` — шаблон DNS-имени с двумя подстановками `%s`. Первая заменяется именем экземпляра, вторая — неймспейсом. Например, с шаблоном `%s.%s.apps.example.com` экземпляр `grafana` в неймспейсе `monitoring` получает имя хоста `grafana.monitoring.apps.example.com`. Если параметр не задан, не создавайте Ingress.
- `ingressClass` — IngressClass для Ingress.
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

Заготовка пакета, созданная командой `d8 package bootstrap app`, содержит хелперы для этих параметров в `templates/_helpers/`: `public_domain`, `ingress_class`, `https_mode`, `https_ingress_tls_enabled` и `https_cert_manager_cluster_issuer_name`.

## Адреса приложения

Чтобы адреса Ingress отображались в поле `status.urls` ресурса Application (например, веб-интерфейс показывает их как ссылки на приложение), добавьте к Ingress аннотацию `packages.deckhouse.io/application-endpoint-description`:

- Значение аннотации — описание адреса. Значение `"true"` добавляет адрес без описания, а значение `"false"` исключает Ingress.
- DP формирует URL для каждой пары `host` и `path` из `spec.rules`. Схема — `https`, если хост указан в `spec.tls`, иначе `http`. Правило без путей даёт URL `<SCHEME>://<HOST>/`. Правила без `host` пропускаются.
- Учитываются только Ingress группы API `networking.k8s.io`, отрендеренные из шаблонов.
- DP обновляет `status.urls` после каждого успешного применения шаблонов.

Пример статуса Application с Ingress из предыдущего раздела:

```yaml
status:
  urls:
    - url: https://myapp.my-namespace.apps.example.com/
      description: Web UI
```

## Ограничения

Шаблоны должны соблюдать [ограничения Application](concepts.html#ограничения-application):

- Создавайте только объекты уровня неймспейса. Не задавайте `metadata.namespace`: DP создаёт объекты в неймспейсе Application, а `d8 package verify` сообщает об ошибке для объектов с этим полем (правило `instance-namespace`).
- Не создавайте объекты CustomResourceDefinition. DP не устанавливает CRD из каталога `crds/` чарта.

По умолчанию `d8 package verify` также требует, чтобы:

- у каждого Deployment и StatefulSet был PodDisruptionBudget, селектор которого соответствует меткам его подов (правило `pdb`);
- у каждого Deployment, StatefulSet и DaemonSet был VerticalPodAutoscaler с политикой для каждого контейнера (правило `vpa`). Тип VerticalPodAutoscaler доступен, только если включён модуль `vertical-pod-autoscaler`, поэтому рендерите его при условии `.Capabilities.APIVersions.Has "autoscaling.k8s.io/v1/VerticalPodAutoscaler"`, как это сделано в заготовке пакета;
- в Service порты контейнеров указывались в `targetPort` по имени (правило `service-port`).

Строгость этих правил можно понизить в `.pkglint.yaml` (см. [«Проверка пакета»](application-development.html#проверка-пакета)).

## Состояние рабочих нагрузок

DP определяет, работает ли приложение, только по его Deployment и StatefulSet:

- Условие `Scaled` ресурса Application становится `True`, когда все Deployment и StatefulSet, отрендеренные из шаблонов, завершили развёртывание и имеют нужное количество готовых реплик. DaemonSet, Job, CronJob и поды не учитываются.
- Deployment, развёртывание которого превысило `spec.progressDeadlineSeconds`, переводит `Scaled` в `False` с причиной `Degraded`.
- Первая установка считается завершённой (`Installed=True`) только после применения манифестов и перехода `Scaled` в `True`.

{% endraw %}
{% alert level="warning" %}
Пакет должен содержать хотя бы один Deployment или StatefulSet. Иначе условие `Scaled` остаётся в состоянии `Unknown`, и приложение никогда не переходит в состояния `Installed` и `Ready`.
{% endalert %}
