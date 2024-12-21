# Changelog v1.68

## [MALFORMED]


 - #10445 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10446 invalid impact level "default | high | low", invalid type "fix | feature | chore", unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #10902 missing section, missing summary, missing type, unknown section ""
 - #11148 unknown section "chore"

## Know before update


 - All authenticators deployed by deckhouse will inherit the highAvailable option from the module instead of using the highAvailable option value of the user-authn. It means that if, e.g., the prometheus module is running in non HA mode, the DexAuthenticator instance will also be deployed without HA.

## Features


 - **[candi]** Added a way to control node labels from files, stored in local directory and it subdirectories. [#10905](https://github.com/deckhouse/deckhouse/pull/10905)
 - **[extended-monitoring]** Added IAM authentication support for ECR in image-availability-exporter. [#10893](https://github.com/deckhouse/deckhouse/pull/10893)
 - **[log-shipper]** Add keepDeletedFilesOpenedFor option. Now it is possible to configure how long Vector keeps deleted file in case of unavailability of a log storage (when Vector cannot send logs to a storage and the internal buffer is already filled). Before the change, Vector used to hold log files opened indefinitely, which can cause a node outage by flooding the disk space. The option makes this behaviour configurable. [#10641](https://github.com/deckhouse/deckhouse/pull/10641)
 - **[user-authn]** Add HA option to DexAuthenticator CRD. [#11049](https://github.com/deckhouse/deckhouse/pull/11049)
    All authenticators deployed by deckhouse will inherit the highAvailable option from the module instead of using the highAvailable option value of the user-authn. It means that if, e.g., the prometheus module is running in non HA mode, the DexAuthenticator instance will also be deployed without HA.

## Chore


 - **[control-plane-manager]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[dhctl]** Replace Logrus to slog implementation, delete 'simple' logger. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
 - **[ingress-nginx]** Disable VPA for Istio sidecar [#11084](https://github.com/deckhouse/deckhouse/pull/11084)
 - **[ingress-nginx]** Added a release requirement check for defaultControllerVersion parameter. [#10941](https://github.com/deckhouse/deckhouse/pull/10941)
 - **[node-local-dns]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[openvpn]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.
 - **[registry-packages-proxy]** Replace Logrus to slog implementation. [#10909](https://github.com/deckhouse/deckhouse/pull/10909)
    Restart components.

