---
Title: Kubectl plugins manager 
---

External-модуль для управления своими [kubectl-плагинами](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) через CustomResource [KubectlPlugin](https://fox.flant.com/team/oscar/external-modules-kubectl-plugins/-/blob/main/example/cr-example.yaml).

- [Возможности](https://fox.flant.com/team/oscar/external-modules-kubectl-plugins#%D0%B2%D0%BE%D0%B7%D0%BC%D0%BE%D0%B6%D0%BD%D0%BE%D1%81%D1%82%D0%B8)
- [Примеры и описание встроенных плагинов](https://fox.flant.com/team/oscar/external-modules-kubectl-plugins#%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80%D1%8B-%D0%B8-%D0%BE%D0%BF%D0%B8%D1%81%D0%B0%D0%BD%D0%B8%D0%B5-%D0%B2%D1%81%D1%82%D1%80%D0%BE%D0%B5%D0%BD%D0%BD%D1%8B%D1%85-%D0%BF%D0%BB%D0%B0%D0%B3%D0%B8%D0%BD%D0%BE%D0%B2)
- [Установка](https://fox.flant.com/team/oscar/external-modules-kubectl-plugins#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0)
- [Структура репозитория](https://fox.flant.com/team/oscar/external-modules-kubectl-plugins#%D1%81%D1%82%D1%80%D1%83%D0%BA%D1%82%D1%83%D1%80%D0%B0)

## Возможности:

Плагин устанавливает на мастера агент в виде daemonset'а, который подписывается на изменения CustomResource KubectlPlugin. Пример CR:
```yaml
apiVersion: d8.externalmodule.io/v1
kind: KubectlPlugin
metadata:
  name: exmaple-plugins
spec:
  plugins:
  - name: foo
    command: |-
      #!/bin/bash

      # optional argument handling
      if [[ "$1" == "version" ]]
      then
          echo "1.0.0"
          exit 0
      fi

      # optional argument handling
      if [[ "$1" == "config" ]]
      then
          echo "$KUBECONFIG"
          exit 0
      fi

      echo "I am a plugin named kubectl-foo"
  - name: ging
    command: |-
      #!/bin/bash
      kubectl -n d8-monitoring get ing -l app=grafana
```
Агенты сохраняют содержащие в СR плагины в каталог: `/usr/local/bin/kubectl-plugins/` и синхронизируют их состояние. А также устанавливают ряд встроенных в модуль плагинов. Отключить установку встроенных плагинов можно через параметр ModuleConfig `spec.settings.defaultPlugins: false`, по умолчанию `true`). 

## Примеры и описание встроенных плагинов
  - ### d8-plugin
    Плагин для упрощения взаимодействий с deckhouse:
    ```bash
    Kubectl plugin for deckhouse.

    Examples:
      # Display table with deckhouse ingress.
      kubectl d8

      # Dump tasks in all deckhouse queues.
      kubectl d8 queue

      # Print the logs for deckhouse.
      kubectl d8 logs [flags]

    Usage:
      d8 [command] [flags]

    Available Commands:
      logs        show deckhouse logs
      queue       how deckhouse queues

    Flags:
          --context string      The name of the kubeconfig context to use
      -h, --help                help for d8
          --kubeconfig string   Path to the kubeconfig file to use for CLI requests.
      -v, --version             version for d8

    Use "d8 [command] --help" for more information about a command.
    ```
    <img src="example/d8.png" width="700">
  - ### broken
    Плагин для поиска подов в "не здоровом статусе":
    ```console
    [mega-production] root@sandbox-master-0 ~ # kubectl broken -A
    NAMESPACE                            NAME                                                              READY   STATUS             RESTARTS          AGE
    default                              nginx-broken                                                      0/1     Running            0                 3m42s
    default                              nginx-broken-2                                                    1/2     Running            0                 3m42s
    default                              nginx-broken-3                                                    1/2     CrashLoopBackOff   5 (15s ago)       3m42s
    default                              nginx-broken-4                                                    0/2     Error              5 (108s ago)      3m42s
    default                              nginx-unscheduled                                                 0/1     Pending            0                 51s
    dev                                  test-6645f4b985-rbm7z                                             0/1     ImagePullBackOff   0                 3d22h
    ```
  - ### dlogs
    Плагин для вывода логов пода deckhouse в красивом форматированном виде:

    <img src="example/dlogs.png" width="500">

  - ### events
    Отображения событий с сортировкой по времени:

    ```console
    [static] root@main-master-0:~# kubectl events
    LAST SEEN   TYPE      REASON                    OBJECT                                         MESSAGE
    28m         Normal    ScaleDown                 node/main-prod-worker-c-afcb0556-8759b-2t4nc   marked the node as toBeDeleted/unschedulable
    27m         Normal    RemovingNode              node/main-prod-worker-c-afcb0556-8759b-2t4nc   Node main-prod-worker-c-afcb0556-8759b-2t4nc event: Removing Node main-prod-worker-c-afcb0556-8759b-2t4nc from Controller
    21m         Normal    Starting                  node/main-prod-worker-c-afcb0556-8759b-f8f99
    20m         Normal    Starting                  node/main-prod-worker-c-afcb0556-8759b-f8f99   Starting kubelet.
    20m         Normal    NodeHasSufficientPID      node/main-prod-worker-c-afcb0556-8759b-f8f99   Node main-prod-worker-c-afcb0556-8759b-f8f99 status is now: NodeHasSufficientPID
    20m         Normal    NodeHasNoDiskPressure     node/main-prod-worker-c-afcb0556-8759b-f8f99   Node main-prod-worker-c-afcb0556-8759b-f8f99 status is now: NodeHasNoDiskPressure
    20m         Normal    NodeAllocatableEnforced   node/main-prod-worker-c-afcb0556-8759b-f8f99   Updated Node Allocatable limit across pods
    20m         Warning   InvalidDiskCapacity       node/main-prod-worker-c-afcb0556-8759b-f8f99   invalid capacity 0 on image filesystem
    20m         Normal    NodeHasSufficientMemory   node/main-prod-worker-c-afcb0556-8759b-f8f99   Node main-prod-worker-c-afcb0556-8759b-f8f99 status is now: NodeHasSufficientMemory
    9m26s       Normal    ScaleDown                 node/main-prod-worker-c-afcb0556-8759b-f8f99   marked the node as toBeDeleted/unschedulable
    8m15s       Normal    RemovingNode              node/main-prod-worker-c-afcb0556-8759b-f8f99   Node main-prod-worker-c-afcb0556-8759b-f8f99 event: Removing Node main-prod-worker-c-afcb0556-8759b-f8f99 from Controller
    ```
  - ### pp
    Плагин для форматированного вывода yaml-манифеста без managedFields.

    <img src="example/pp.png" width="500">
    
    !!для работы необходим yq
  - ### lp (list pods )
    Плагин для отображения информации по подам: 
    ```bash
    Display list pods.

    Examples:                              #  [- shortcuts -]
      # show pods with resources.               [ res, r ]
      kubectl lp resource                       

      # display pods antiaffinity.              [ aaf, af ]
      kubectl lp antiaffinty                   

      # display pods tolerations.               [ tol, t ]
      kubectl lp tolerations                    

      # display pods nodeaffinity.              [ naf, n ]
      kubectl lp nodeaffinity                   

    Usage:
      lp [command] [flags] 

    Available Commands:
      antiaffinty    show pods antiaffinity
      nodeaffinity   show pods node affinity
      resource       show pods with resources
      tolerations    show pods tolerations

    Flags:
          --context string      The name of the kubeconfig context to use
      -h, --help                help for lp
          --kubeconfig string   Path to the kubeconfig file to use for CLI requests.
      -n, --namespace string    If present, the namespace scope for this CLI request

    Use "lp [command] --help" for more information about a command.
    ```
    ```console
    [team-echo] root@sandbox-master-0 ~ # kubectl lp r
          NAME       	READY	     STATUS     	RESTARTS	MEMREQ	MEMLIM	CPUREQ	CPULIM	AGE
    backend-main     	 1/1 	    Running     	   0    	64Mi  	 64Mi 	1m    	  0   	63m
    nginx-broken     	 0/1 	    Running     	   0    	64Mi  	 64Mi 	1m    	  0   	63m
    nginx-broken-2   	 1/2 	    Running     	   0    	128Mi 	128Mi 	2m    	  0   	63m
    nginx-broken-3   	 1/2 	     Error      	   17   	128Mi 	128Mi 	2m    	  0   	63m
    nginx-broken-4   	 0/2 	CrashLoopBackOff	   16   	128Mi 	128Mi 	2m    	  0   	63m
    nginx-unscheduled	 0/1 	    Pending     	   0    	64Mi  	 64Mi 	1m    	  0   	60m 
    ```
    ```console
    [team-echo] root@sandbox-master-0 ~ # kubectl lp naf --prefix nginx-uns
            POD          READY   STATUS    AGE                      NODEAFFINITY                     
    nginx-unscheduled     0/1    Pending   43m   requiredduringschedulingignoredduringexecution:       
                                                    nodeselectorterms:                                
                                                        - matchexpressions:                           
                                                            - key: kubernetes.io/os                   
                                                              operator: In                            
                                                              values:                                 
                                                                - notlinux                            
                                                          matchfields: []                             
                                                 preferredduringschedulingignoredduringexecution: []   
    ```
## Установка:
1. Подключаем репозиторий с модулем при помощи создания объекта `ModuleSource` или выкатываем его из репы:
```yaml
cat <<EOF | kubectl apply -f -
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: kubectl-plugins
spec:
  releaseChannel: stable
  registry:
    repo: registry.flant.com/team/oscar/external-modules-kubectl-plugins
    dockerCfg: ewogICJhdXRocyI6IHsKICAgICJyZWdpc3RyeS5mbGFudC5jb20iOiB7CiAgICAgICJhdXRoIjogImNtOWZjSFZzYkdWeU9uUnJOamszYlhvMWRsOHhRMUZNZEUxQmQzVkgiCiAgICB9CiAgfQp9Cg==
EOF
```
2. Включаем модуль при помощи ModuleConfig:
```yaml
cat <<EOF | kubectl apply -f -
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kubectl-plugins
spec:
  enabled: true
  settings:
    defaultPlugins: true
  version: 1
EOF
```
3. Ждем появления агентов в ns `kubectl-plugins`:
```console
[team-echo] root@sandbox-master-0 ~ # kubectl -n kubectl-plugins get pods
NAME                          READY   STATUS    RESTARTS   AGE
kubectl-plugins-agent-bvvg8   1/1     Running   0          40s
```
4. Profit!

## Структура:

`apps` - каталог с кодом агента и дефолтными плагинами.

`crds` - папка содержащая описание кастомных ресурсов проекта.

`example` - папка содержащая примеры CR.

`hooks` - папка с хуками модуля. Подробнее о хуках и подписках читать [тут](https://github.com/flant/shell-operator).

`images` - папка с сборкой образов, поддерживается сборка из `Dockerfile` или из `werf.inc.yaml`.

`openapi` - папка с openapi-описанием values модуля (конфигурационных и runtime).

`lib` - папка с зависимостями python для хуков.

`templates` - Helm-темплейты для деплоя компонентов модуля.

`Chart.yaml` - файл описания модуля.

----
По любым вопросам по работе, доработкам, предложениям обращаться к:
```
maintainers:
- name: Nikolay Gorbatov
  email: nikolay.gorbatov@flant.com
  slack: @nikolay.gorbatov
- name: Anton Kulyashov
  email: anton.kulyashov@flant.com
  slack: @anton.kulyashov
```