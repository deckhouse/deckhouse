## far-converter

Converts files with Falco rules to FalcoAuditRules CRD format.

### Usage:

Stdout:
```shell
go run main.go -input /path/to/falco/rules.yaml
```

To a file:
```shell
go run main.go -input /path/to/falco/rules.yaml > ./my-rules.yaml
```
