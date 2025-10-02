# dvcr-cleaner

## Использование

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

## Как получить список всех виртуальных образов

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

## Как получить виртуальный образ "head-1f8f5e16-vi-alpine-http" в пространстве имен "head-1f8f5e16-testcases"

```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner ls vi head-1f8f5e16-vi-alpine-http --namespace head-1f8f5e16-testcases
Name                         
head-1f8f5e16-vi-alpine-http 
```

## Как удалить виртуальный образ "head-1f8f5e16-vi-alpine-http" в пространстве имен "head-1f8f5e16-testcases"

```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner delete vi head-1f8f5e16-vi-alpine-http --namespace head-1f8f5e16-testcases
? Confirm? [y/N] y█
Successful
```

## Сборщик мусора

Когда все операции по удалению образов завершены, необходимо запустить `garbage collect`:
```bash
k exec -it -n d8-virtualization dvcr-84c4bffc46-tkgvq -c dvcr -- dvcr-cleaner gc run
? Confirm? [y/N] y█
```

https://github.com/distribution/distribution/issues/1803
```bash
k rollout restart deployment -n d8-virtualization dvcr
```
