# dvcr-cleaner

## Usage

```bash
$ k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner
`dvcr-cleaner` is used for exploring and removing `VirtualImages` and `ClusterVirtualImages` from registry.

Usage:
  dvcr-cleaner [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  delete      Delete `VirtualImages` or `ClusterVirtualImages`
  gc          Garbage collector
  help        Help about any command
  ls          A list of `VirtualImages` or `ClusterVirtualImages`

Flags:
  -h, --help   help for dvcr-cleaner

Use "dvcr-cleaner [command] --help" for more information about a command.
```

## How to list of all `VirtualImages`

```bash
$ k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner ls vi --all-namespaces
Namespace                Name                                      
head-1f8f5e16-testcases  head-1f8f5e16-jammy-server-cloudimg-amd64 
head-1f8f5e16-testcases  head-1f8f5e16-vi-alpine-http              
head-1f8f5e16-testcases  head-1f8f5e16-vi-alpine-registry          
head-1f8f5e16-testcases  head-1f8f5e16-vi-from-cvi-alpine-http     
head-1f8f5e16-testcases  head-1f8f5e16-vi-from-vi-alpine-http      
head-24a6224c-testcases  head-24a6224c-vi-alpine-http              
head-24a6224c-testcases  head-24a6224c-vi-alpine-registry          
head-24a6224c-testcases  head-24a6224c-vi-from-vi-alpine-http      
head-38213649-testcases  head-38213649-vi-alpine-http              
head-46816b0c-testcases  head-46816b0c-jammy-server-cloudimg-amd64 
head-46816b0c-testcases  head-46816b0c-vi-alpine-http              
...
```

## How to get the `VirtualImage` "head-1f8f5e16-vi-alpine-http" in the "head-1f8f5e16-testcases" namespace

```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner ls vi head-1f8f5e16-vi-alpine-http --namespace head-1f8f5e16-testcases
Name                         
head-1f8f5e16-vi-alpine-http 
```

## How to delete the `VirtualImage` "head-1f8f5e16-vi-alpine-http" in the "head-1f8f5e16-testcases" namespace

```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner delete vi head-1f8f5e16-vi-alpine-http --namespace head-1f8f5e16-testcases
? Confirm? [y/N] y█
Successful
```

## Garbage collector

When all removing operations are finished, run the `garbage collect`:
```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner gc run
? Confirm? [y/N] y█
```

https://github.com/distribution/distribution/issues/1803
```bash
k rollout restart deployment -n d8-virtualization dvcr
```
