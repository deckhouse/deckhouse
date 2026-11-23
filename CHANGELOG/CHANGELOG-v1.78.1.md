# Changelog v1.78.1

## Know before update


 - An update from Deckhouse 1.77 with the PSS requirement check is blocked while admission-policy-engine uses an implicit Privileged policy.
    Before updating, explicitly set settings.podSecurityStandards.defaultPolicy in the admission-policy-engine ModuleConfig.
    Set it to Privileged to keep the current policy. Verify that workloads comply before choosing Baseline or Restricted.

## Fixes


 - **[admission-policy-engine]** Activate the PSS default policy requirement for updates from Deckhouse 1.77 to 1.78.1.
    An update from Deckhouse 1.77 with the PSS requirement check is blocked while admission-policy-engine uses an implicit Privileged policy.
    Before updating, explicitly set settings.podSecurityStandards.defaultPolicy in the admission-policy-engine ModuleConfig.
    Set it to Privileged to keep the current policy. Verify that workloads comply before choosing Baseline or Restricted.
