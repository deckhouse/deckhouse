---
title: Queuing mechanism
permalink: en/architecture/deckhouse/queues.html
search: deckhouse, deckhouse-controller, modules, queue
description: Description of queue processing in the Deckhouse controller in Deckhouse Kubernetes Platform.
---

The [`deckhouse`](/modules/deckhouse/) module implements the core of Deckhouse Kubernetes Platform (DKP), performing various platform management operations using a queueing mechanism. For more information about the module architecture, refer to the [corresponding documentation section](./deckhouse.html).

The Deckhouse controller implements addon-operator and marketplace queues.

### Addon-operator queues

The **addon-operator queues** are the primary processing mechanism for built-in and external Deckhouse modules. The queue is implemented in [shell-operator](https://github.com/flant/shell-operator) and extended with [addon-operator](https://github.com/flant/addon-operator) task types. The Deckhouse controller synchronizes [ModuleConfig](../../reference/api/cr.html#moduleconfig) custom resources and updates global or module values for addon-operator.

Task types:

| Task                               | Function                                                                            |
|------------------------------------|-------------------------------------------------------------------------------------|
| GlobalHookRun                      | Run global hooks (onStartup, beforeAll, afterAll, kubernetes, schedule)           |
| GlobalHookEnableKubernetesBindings | Enable Kubernetes monitors from global hooks                                         |
| GlobalHookWaitKubernetesSynchronization | Block the queue until global hooks with `executeHookOnSynchronization: true` complete |
| GlobalHookEnableScheduleBindings   | Register cron tasks in the addon-operator scheduler                                 |
| DiscoverHelmReleases               | Find "extra" Helm releases after the first converge                                |
| ApplyKubeConfigValues              | Apply changes from ModuleConfig                                                     |
| ConvergeModules                    | Full converge cycle for all modules                                                 |
| ModuleRun                          | Configure or update a module; runs subtasks: onStartup -> sync -> beforeHelm -> helm -> afterHelm |
| ParallelModuleRun                  | Batch parallel launch of modules                                                    |
| ModuleDelete                       | Remove a module (helm delete, afterDeleteHelm)                                      |
| ModuleHookRun                      | Run a module hook on event                                                          |
| ModuleEnsureCRDs                   | Install module CRDs                                                                 |
| ModulePurge                        | Remove unknown Helm release                                                         |

Addon-operator queue types:

| Queue       | Name                                   | Purpose                                                                                      |
|-------------|----------------------------------------|----------------------------------------------------------------------------------------------|
| Main        | `main`                                 | Run global hooks at startup, install critical modules, configure and delete modules          |
| Parallel    | `parallel_queue_0` ... `parallel_queue_19` | Parallel ModuleRun with module dependencies taken into account (20 queues)                  |
| Hook queues | From task config ([hook](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md)) | Tasks for a specific module and global hooks |

Each queue is a separate pipeline with one worker. The queue has the following properties:

- Tasks can be inserted both at the tail (`AddLast`) and at the head (`AddFirst`) of the queue.
- Tasks are executed from the head of the queue.
- Multiphase operations are supported with the following operations:
  - `AddHeadTasks`: Insert subtasks before the current task.
  - `AddTailTasks`: Insert a subtask at the end of the queue after the current task succeeds.
  - `AddAfterTasks`: Insert a subtask right after the current task.
- A task is executed until success unless `allowFailure: true` is set in task parameters.
- On error, exponential restart (backoff) is applied, starting with a 5-second delay between retries.

{% alert level="warning" %}
If a task cannot complete successfully and `allowFailure: true` is not set in its parameters, that task blocks the queue it runs in.
Tasks in different processing queues do not block each other.
{% endalert %}

At startup, the Deckhouse controller creates `main` and `parallel_queue_0..19` queues and adds the following tasks to `main` in this order:

- GlobalHookRun (onStartup): For each global hook.
- GlobalHookEnableScheduleBindings: To enable cron scheduling.
- GlobalHookEnableKubernetesBindings: To enable global tasks that watch Kubernetes resources.
- GlobalHookWaitKubernetesSynchronization.
- ConvergeModules (OperatorStartup): First converge of all modules.

After ConvergeModules, the controller adds the DiscoverHelmReleases task to clean up unknown Helm releases.

The module processing order in the ConvergeModules task is determined by several attributes:

- Module criticality: The `critical` parameter in the module `module.yaml` configuration.
- Module weight: Numeric module processing order. The higher the number, the later the module is processed. The weight is taken from the `weight` parameter in module `module.yaml`; if missing or set to 0, the default weight 900 is used. If no module config file exists, the weight is taken from the numeric prefix of the module directory name (for example, `040-node-manager` means weight 40). If the weight cannot be obtained from the directory name, weight 100 is used.
- Module dependencies: A list of modules that must be installed before the current module.

Based on these attributes, the Deckhouse controller scheduler defines processing order according to the following principles:

- For critical modules, module weight is considered in ascending order and tasks are put into the `main` queue.
- For non-critical modules, module weight is not considered and tasks are put into `parallel_queue_0..19`.
- For all modules, dependencies on other modules are considered.

If critical modules can be processed in parallel, the Deckhouse controller scheduler places the ParallelModuleRun task into the `main` queue with the list of modules. The ParallelModuleRun task starts a ModuleRun task for each module in `parallel_queue_0..19` and waits for completion, thereby blocking the `main` queue. If an error occurs while processing a ModuleRun task, the scheduler moves this task to the end of the queue and starts the next queue task.

The process of installing critical modules is shown in the following diagram:

![Sequence diagram for installing critical modules](../../images/architecture/deckhouse/DECKHOUSE_QUEUE_MODULES_CRITICAL.svg)

The process of installing non-critical (functional) modules is shown in the following diagram:

![Sequence diagram for installing functional modules](../../images/architecture/deckhouse/DECKHOUSE_QUEUE_MODULES_FUNCTIONAL.svg)

If more than one identical task is added to a queue, all duplicates are removed when such a task starts, as part of deduplication.

To view addon-operator queues, use the `d8 system queue list` command.

### Marketplace queues

The **Marketplace queues** are a queue implementation used by [Marketplace](../marketplace) functionality.

Each queue served by the Deckhouse controller for Marketplace has the following properties:

- FIFO (First In First Out): Defines strict task execution order. The first task in queue is executed first.
- Strict sequential execution of tasks, one task at a time.
- Tasks are started only on events (event-driven), without polling any resources.
- Restart on errors with exponential backoff between retries, starting at 15 seconds and capped at 1 minute, with no limit on retry attempts.
- Supports cascading task cancellation on version changes or package deletion.

Queue types:

| Name                           | Purpose                                                           |
|--------------------------------|-------------------------------------------------------------------|
| {packageName}                  | Lifecycle: Deploy, Load, Configure, Enable, Run, Disable, Undeploy |
| {packageName}/{hookQueue}      | Hooks for K8s/schedule events (queue from hook binding)          |
| {packageName}/{hookQueue}/sync | Hook synchronization at startup (WaitForSynchronization)         |

Task types:

| Task      | Function                                                                                     |
|-----------|----------------------------------------------------------------------------------------------|
| Deploy    | Downloads/mounts the package image                                                           |
| Load      | Parses configuration, creates Application/Module, registers in scheduler                     |
| Configure | Applies Application/Module settings using the parameter store                                |
| Enable    | Enables hooks, performs parameter synchronization, runs OnStartup hook                       |
| Run       | Runs subtasks when installing Application/Module: BeforeHelm -> helm Upgrade -> AfterHelm    |
| HookRun   | Runs a hook on event                                                                         |
| HookSync  | Initial synchronization of Kubernetes binding                                                |
| Disable   | Removes Helm, disables hooks, clears hook queues                                             |
| Undeploy  | Removes the package from disk                                                                |

Task execution in one queue does not block task execution in another queue.

To view Marketplace queues, use the `d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages queue dump` command.

{% alert level="warning" %}
When performing module installation or configuration tasks in addon-operator, the Deckhouse controller pauses Marketplace queue processing.
{% endalert %}
