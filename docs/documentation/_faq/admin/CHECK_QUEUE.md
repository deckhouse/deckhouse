---
title: How to check the job queue in Deckhouse?
subsystems:
  - deckhouse
lang: en
---

#### How to check the status of all Deckhouse task queues?

To view the status of all Deckhouse job queues, run the following command:

```shell
d8 s queue list
```

Example output (queues are empty):

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

#### How to view the status of the main task queue?

To view the status of the Deckhouse `main` task queue, run the following command:

```shell
d8 s queue main
```

Example output (38 tasks in the `main` queue):

```console
Queue 'main': length 38, status: 'run first task'
```

Example output (the `main` queue is empty):

```console
Queue 'main': length 0, status: 'waiting for task 0s'
```
