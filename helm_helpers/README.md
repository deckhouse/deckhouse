For updating deckhouse helm lib use next instruction:
1. Change version in `Chart.yaml`
2. Use next command to apply changes:
```bash
helm dependency update && tar -xf charts/deckhouse_lib_helm-*.tgz -C charts/
```
3. And commit all changes in the repository:
```bash
git add Chart.yaml Chart.lock charts/*
```
