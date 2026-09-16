# cloud

Hooks of the cloud providers team. Their subject is a specific cloud or the cloud-data reconciler those teams own. node-manager only hosts the files.

- `check_unmet_conditions` copies the unmet conditions reported by the cloud provider into the release requirements store, so a Deckhouse upgrade waits until the cloud side is ready.
- `yc_delete_preemptible_instances` deletes Yandex Cloud preemptible machines older than 24 hours at a moment of our choosing, before the cloud reclaims them at an inconvenient one.
