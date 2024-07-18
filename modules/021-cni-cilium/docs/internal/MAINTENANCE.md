# Maintenance

## Rollout restarting of the cilium agent pods

```bash
kubectl -n d8-cni-cilium annotate pod -l app=agent safe-agent-updater-daemonset-generation- && kubectl -n d8-cni-cilium rollout restart ds safe-agent-updater
```
