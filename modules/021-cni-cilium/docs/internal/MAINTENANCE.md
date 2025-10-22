# Maintenance

## Rollout restarting of the cilium agent pods

Previously, a mechanism was added to safely update pods with cilium agents. Because of this, the simple `rollout restart ds agent` command no longer functions.

The following command restarts all pods of the cilium agent sequentially, if necessary:

```bash
kubectl -n d8-cni-cilium annotate pod -l app=agent safe-agent-updater-daemonset-generation- && kubectl -n d8-cni-cilium rollout restart ds safe-agent-updater
```
