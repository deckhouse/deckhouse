---
title: "The flow-schema module: FAQ"
---

## How to check priority levels state?

Execute:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_priority_levels
```

## How to check detailed info about priority level queues?

Execute:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_queues
```

## Useful metrics

- `apiserver_flowcontrol_rejected_requests_total` — the number of rejected requests.
- `apiserver_flowcontrol_dispatched_requests_total` — the number of requests already been handled.
- `apiserver_flowcontrol_current_inqueue_requests` — the number of pending requests in the queue.
- `apiserver_flowcontrol_current_executing_requests` — the number of requests in processing.
