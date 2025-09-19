# infra-controller-manager

## Local run 

```bash
VCD_TOKEN=<YOUR_TOKEN_HERE> VCD_ORG=<YOUR_ORG_HERE> VCD_VDC=<YOUR_VCD_HERE> VCD_VAPP=<YOUR_VAPP_NAME_HERE> VCD_HREF=<YOUR_VCD_URL_HERE> go run ./...
```

## CRD Maintenance

Work with resources here: `api/v1alpha1/*_types.go`

Generate deepcoopy functions: 

```bash
make generate
```

Update CRD: 

```
make manifests
```

## License

Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
