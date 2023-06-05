---
title: "Модуль multitenancy-manager: FAQ"
---

## Как это работает?

Для каждого [Project](cr.html#project) происходит создание отдельного `Namespace` с именем ресурса [Project](cr.html#project), создание [AuthorizationRule](../../modules/140-user-authz/cr.html#authorizationrule) из приведенных данных в поле [subjects](cr.htlm#projecttype-v1alpha1-spec-subjects) ресурса [ProjectType](cr.htlm#projecttype) и рендер темлпейтов ([ProjectType](cr.htlm#projecttype) поле [resourcesTemplate](cr.htlm#projecttype-v1alpha1-spec-resourcestemplate)) ресурсов с помощью `helm` из предоставленных значений в [template](cr.htlm#project-v1alpha1-spec-template), которые валидируются на сопоставление OpenAPI спецификации из [ProjectType поля openAPI](cr.htlm#projecttype-v1alpha1-spec-openapi).
