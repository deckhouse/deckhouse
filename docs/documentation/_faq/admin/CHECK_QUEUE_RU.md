---
title: Как проверить очередь заданий в Deckhouse?
subsystems:
  - deckhouse
lang: ru
---

#### Как посмотреть состояние всех очередей заданий Deckhouse?

Для просмотра состояния всех очередей заданий Deckhouse выполните следующую команду:

```shell
d8 s queue list
```

Пример вывода (очереди пусты):

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

#### Как посмотреть состояние очереди заданий main?

Для просмотра состояния очереди заданий `main` Deckhouse выполните следующую команду:

```shell
d8 s queue main
```

Пример вывода (в очереди `main` 38 заданий):

```console
Queue 'main': length 38, status: 'run first task'
```

Пример вывода (очередь `main` пуста):

```console
Queue 'main': length 0, status: 'waiting for task 0s'
```
