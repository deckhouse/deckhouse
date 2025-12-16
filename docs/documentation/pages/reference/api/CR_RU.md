---
title: "Custom Resources"
permalink: ru/reference/api/cr.html
lang: ru
search: global custom resources, global configuration, global parameters, глобальные настройки, настройки платформы, глобальные параметры
---

{{ site.data.schemas.crds.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.crds.deckhouse-release | format_crd: "global" }}

{{ site.data.schemas.crds.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.crds.module | format_crd: "global" }}
{{ site.data.schemas.crds.module-config | format_crd: "global" }}
{{ site.data.schemas.crds.module-documentation | format_crd: "global" }}
{{ site.data.schemas.crds.module-pull-override | format_crd: "global" }}
{{ site.data.schemas.crds.module-release | format_crd: "global" }}
{{ site.data.schemas.crds.module-settings-definition | format_crd: "global" }}
{{ site.data.schemas.crds.module-source | format_crd: "global" }}
{{ site.data.schemas.crds.module-update-policy | format_crd: "global" }}

{{ site.data.schemas.crds.objectkeeper | format_crd: "global" }}

{{ site.data.schemas.crds.static_cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.crds.ssh_configuration | format_cluster_configuration }}
{{ site.data.schemas.crds.ssh_host_configuration | format_cluster_configuration }}
