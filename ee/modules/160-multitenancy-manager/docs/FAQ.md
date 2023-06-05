---
title: "The multitenancy-manager module: FAQ"
---

## How it works?

For each [Project](cr.html#project) resource, an individual `Namespace` is created with the name of the [Project](cr.html#project) resource, an [AuthorizationRule](../../modules/140-user-authz/cr.html#authorizationrule) is created from the provided data in [subjects](cr.htlm#projecttype-v1alpha1-spec-subjects) field of the [ProjectType](cr.htlm#projecttype) and resources templates ([resourcesTemplate](cr.htlm#projecttype-v1alpha1-spec-resourcestemplate) field from [ProjectType](cr.htlm#projecttype)) are rendered using `helm` from provided values in [template](cr.htlm#project-v1alpha1-spec-template) which are validated against OpenAPI requirements from [ProjectType openAPI](cr.htlm#projecttype-v1alpha1-spec-openapi) field.
