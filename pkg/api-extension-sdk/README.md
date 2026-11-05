# api-extension-sdk

Go packages for extension API servers (aggregated APIs) that run in Deckhouse.

```bash
go get github.com/deckhouse/deckhouse/pkg/api-extension-sdk
```

| Package | Contents |
|---|---|
| [`scope`](scope/README.md) | The receiver side of scoped reads (`--scope`): trust of the scope headers sent by kube-apiserver, authorization of opted-in reads and filtering of list, get and watch. |

Releases are tagged `pkg/api-extension-sdk/vX.Y.Z`.
