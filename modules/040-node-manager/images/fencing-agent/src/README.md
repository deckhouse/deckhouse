# fencing-agent

Notes for the developers of the fencing agent.

## Lower bounds of the profile timings

The module ships three FencingSLAProfile objects: `critical`, `normal` and `slow` (`templates/fencing-agent/sla-profiles/`). Users do not create profiles. This section is for the developers who change the built-in profiles.

`Validate` (`internal/usecase/profile/validate.go`) and the CEL rules of the CRD check only some of the bounds below. The other bounds have no check in the code. Do not set a value below its bound.

In the formulas, a profile field has its path in the CRD, for example `poll.witnessFresherBy`. A common timing has its path in the agent ConfigMap, for example `commonTimings.roundTimeout`. The common timings are the same for all profiles. They come from the internal values of the module, `nodeManager.internal.fencing.commonTimings`. A derived value has one name in camelCase, for example `pollBudget`.

### Probe parameters

The bounds use the probe parameters of kubernetes-api-proxy. The proxy uses its default flags on every node: bashible sets only `--health-listen` (`images/kubernetes-api-proxy/src/README.md`, `config.go`). The own probes of the agent (`internal/usecase/cpview/prober.go`) use the same values.

| Name | Meaning | Value | Source |
|---|---|---|---|
| Probe interval | Interval between the starts of two probe rounds | 1 s ±20 % | `--health-interval`, `--health-jitter` |
| `proxyMaxInterval` | Longest time between the starts of two rounds, 1.2 × the probe interval | 1.2 s | `proxyMaxInterval` |
| `proxyTimeout` | Probe timeout | 0.5 s | `--health-timeout`, `proxyTimeout` |
| `failStepInterval` | Fail-step interval, with the same jitter | 0.5 s | `--fail-step-interval` |
| Fail-step window | Duration of fail-step | 30 s | `--fail-step-window` |
| `maxProbeAge` | Largest `lastReadyAgo` when no probe is lost, `proxyMaxInterval + proxyTimeout` | 1.7 s | `MaxProbeAge` |

`lastReadyAgo` is the age of the last `ready` answer: `lastReadyAgoMs` of `/upstreams`, the minimum over the upstreams. The proxy does not retry a failed probe. A round without a `ready` upstream starts fail-step.

`lastReadyAgo` is the minimum over the upstreams, so it is more than maxProbeAge only after a round that lost the probes to all upstreams. The largest `lastReadyAgo` after such a round is 2.3 s:

1. A round starts at s and loses the probes to all upstreams. The last `ready` answer is not older than s − proxyMaxInterval.
1. The round ends at s + proxyTimeout at most and starts fail-step.
1. The next round starts at s + max(proxyTimeout, 1.2 × failStepInterval) = s + 0.6 s. Its answer comes in less than proxyTimeout.
1. Thus lastReadyAgo ≤ proxyMaxInterval + 1.2 × failStepInterval + proxyTimeout = 1.2 + 0.6 + 0.5 = 2.3 s.

Thus the bound of `poll.witnessFresherBy` is proxyMaxInterval + max(proxyTimeout, 1.2 × failStepInterval) + proxyTimeout + 0.05 s. The term proxyTimeout is larger only when failStepInterval is less than 0.42 s.

Two such rounds in sequence give `lastReadyAgo` up to 2.9 s.

### Summary

| Parameter | Bound | Value of the bound | Built-in: critical / normal / slow | Check in the code |
|---|---|---|---|---|
| `commonTimings.fastReadAfter` | fastReadAfter ≥ proxyMaxInterval | 1.2 s | 1.2 s, common | None |
| `poll.witnessFresherBy` | witnessFresherBy ≥ proxyMaxInterval + max(proxyTimeout, 1.2 × failStepInterval) + proxyTimeout + 0.05 s | 2.35 s | 2.35 / 2.35 / 2.35 s | `Validate`: only maxProbeAge < witnessFresherBy < controlPlaneLostAfter − maxProbeAge |
| `controlPlaneLostAfter` | controlPlaneLostAfter ≥ witnessFresherBy + maxProbeAge + 0.1 s | 4.15 s with witnessFresherBy = 2.35 s | 4.15 / 9.1 / 20.2 s | `Validate` |
| `controlPlaneLostAfter` | controlPlaneLostAfter ≥ heartbeat.interval + heartbeat.apiTimeout and controlPlaneLostAfter ≥ 3 × heartbeat.interval | 3 / 7 / 15 s | 4.15 / 9.1 / 20.2 s | `Validate`, CEL |
| pollBudget = 2 × `commonTimings.roundTimeout` + `commonTimings.confirmTimeout` | pollBudget ≥ max(0.6 s, witnessFresherBy + maxProbeAge − controlPlaneLostAfter + 0.1 s) | 0.6 s | 0.9 s, common | `Resolve` of the configuration: pollBudget ≥ 0.6 s. `Validate` makes the second term not more than 0 |
| Proxy `--fail-step-interval` (failStepInterval) | failStepInterval ≤ 0.5 s | 0.5 s | 0.5 s | None |
| Proxy `--fail-step-window` | Not 0 and at least controlPlaneLostAfter + 0.2 s + pollBudget + freshnessWait of each profile | 23.1 s (`slow`) | 30 s | None |

If you change a probe parameter of the proxy, change these places together: the proxy defaults in `config.go`, the constants in `validate.go` and `cpview/prober.go` and the probe values in the description of `crds/fencingslaprofiles.yaml`. Then calculate all the bounds again.

The measurements below come from two clusters of 9 and 8 nodes with the built-in profiles and the proxy defaults.

### `commonTimings.fastReadAfter`

fastReadAfter sets only the period of the reads of `/upstreams`: 200 ms, and 100 ms while lastReadyAgo > fastReadAfter (`cpview/view.go`). No decision uses fastReadAfter.

Below the bound, `lastReadyAgo` of a healthy node is more than fastReadAfter for a large part of the time. Then the agent reads the proxy two times as frequently. In the simulation with a probe interval of 1 s, fastReadAfter = 0.75 s gave fast reads for about 24 % of the time, and fastReadAfter = 1.2 s for about 0.1 %.

Measurement: one hour without load, 17 nodes. `lastReadyAgo` was more than 1.2 s for 0 of the sampled time on 16 nodes and for 1.4·10⁻⁵ of the sampled time on one node. On one cluster the sampler did not see 1.3–4.6 % of the time, in gaps of up to 1.29 s (1.65 s on a master under load).

### `poll.witnessFresherBy`

The agent counts a `see` answer as a witness when its own `lastReadyAgo` minus `lastReadyAgo` of the peer is more than witnessFresherBy (`decision/engine.go`). After two failed own probe rounds (the rule of freshness), the witness makes the agent stop the feed of the watchdog.

When a segment loses the control plane, `lastReadyAgo` of two nodes in it can differ by the largest `lastReadyAgo` of a healthy node: 2.3 s after a round that lost the probes to all upstreams. With witnessFresherBy below 2.35 s, a peer in the same segment can become a witness. The own probes fail too, so the rule of freshness does not cancel the witness. The node then stops the feed of the watchdog by mistake. It must wait for the control plane instead.

In the simulation of a lost segment uplink (17 nodes, one master, 5.2 million events), the share of events with a false stop was:

| witnessFresherBy | 1.54 s | 1.8 s | 2.0 s | 2.29 s | 2.35 s |
|---|---|---|---|---|---|
| Events with a false stop | 6.7·10⁻⁴ | 1.3·10⁻⁵ | 1.5·10⁻⁶ | 0 | 0 |

Measurements:

- One hour without load, 17 nodes: the largest sampled `lastReadyAgo` on a node was 1183–1213 ms. The 99th percentile was 1106–1107 ms. There were 0 lost probes and 0 fail-step episodes. Thus the measurement did not reach the case of a round that lost the probes to all upstreams. The bound 2.35 s comes from the code of the proxy.
- With one master under load, the largest sampled `lastReadyAgo` was 1198 ms.
- A pause of the whole control plane for 3.65–10.1 s and the isolation of a segment: 0 stops of the feed.

witnessFresherBy covers one such round only. A short failure of 0.5–1 s on the node can cause two such rounds in sequence. Then `lastReadyAgo` can be 2.6–3.1 s, which is more than witnessFresherBy.

`Validate` also requires witnessFresherBy < controlPlaneLostAfter − maxProbeAge. With controlPlaneLostAfter = 4.15 s, witnessFresherBy must be less than 2.45 s.

### `controlPlaneLostAfter`

The agent declares the control plane lost when lastReadyAgo > controlPlaneLostAfter. The reads of `/upstreams` come at least every 0.2 s, so the agent declares the loss at `lastReadyAgo` between controlPlaneLostAfter and controlPlaneLostAfter + 0.2 s. Then the agent stops the feed at once, and an armed agent polls the peers for a witness (`decision/engine.go`).

A healthy peer without a lost probe has lastReadyAgo ≤ maxProbeAge, and its answer can come 0.1 s late. With controlPlaneLostAfter ≥ witnessFresherBy + maxProbeAge + 0.1 s, each healthy peer is a witness in the first poll. Below the bound, a node that lost only its own path to the API can find no witness in the poll: a `see` answer that is fresher by witnessFresherBy or less is not a witness (`decision/engine.go`). The stop of the feed does not wait for the poll, so it is not late. Without a witness the node sends no DeclareSuicide, and no peer writes the attestation.

An increase of witnessFresherBy by some time increases the minimum controlPlaneLostAfter and all the times below by the same time.

Time from the failure to the stop of the feed: controlPlaneLostAfter + 0.2 s, for a node that lost only the API and for a node that lost all networks. The poll comes after the stop. A witness may answer as late as the confirming round, at the end of pollBudget. The node sends DeclareSuicide to the witness after freshnessWait, the wait of the rule of freshness: 2 × failStepInterval + proxyTimeout = 1.5 s, and 2 × 1.2 × failStepInterval + proxyTimeout + 0.1 s = 1.8 s in the worst case.

The node stops watchdog.timeout after the stop of the feed. For `critical` the limits of the stop of the feed and of the node are 4.35 s and 9.35 s.

Measurements for `critical`:

- The API of one node is cut, 5 runs: the feed stopped after 4.70–5.39 s, the node stopped after 9.58–10.29 s. `lastReadyAgo` in the log record `control plane is lost` was 4.272–4.348 s.
- One node is isolated from all networks, 4 runs: the node stopped after 8.18–8.60 s.
- The API of one node is dropped for 1, 2 and 3 s, 3 runs each: 0 records `control plane is lost` and 0 stops of the feed. Rounds of fail-step started at least 403 ms apart.
- A pause of the whole control plane: the agents can declare the loss when the pause is longer than controlPlaneLostAfter − maxProbeAge = 2.45 s. This is expected. With a pause of 3.65 s, 6 of 9 agents declared the loss. Shorter pauses were not measured. There were 0 stops of the feed.

Measurements for `normal`: the API of one node is cut, the feed stopped after 10.49 s, the node stopped after 24.88 s. The API is dropped for 5 and 7 s: 0 losses of the control plane.

### pollBudget: the poll budget

At the start of the poll, the own `lastReadyAgo` is at least controlPlaneLostAfter, and it grows during the poll. With pollBudget ≥ witnessFresherBy + maxProbeAge − controlPlaneLostAfter + 0.1 s, the own `lastReadyAgo` reaches witnessFresherBy + maxProbeAge + 0.1 s before the poll ends. Then each healthy peer is a witness. Below the bound, the poll can end without a witness: the node sends no DeclareSuicide, and no peer writes the attestation. The stop of the feed comes before the poll and does not depend on pollBudget. `Validate` requires controlPlaneLostAfter ≥ witnessFresherBy + maxProbeAge + 0.1 s, so this bound is not more than 0 for each valid profile. `Resolve` of the agent configuration requires pollBudget ≥ 0.6 s.

### Delay before the taint of a silent node

The profile has no field for this delay. The fencing controller calculates it: controlPlaneLostAfter + 0.2 s + watchdog.timeout + watchdog.margin + 0.9 s.

### fail-step of the proxy

The bound of witnessFresherBy uses failStepInterval = 0.5 s. With failStepInterval = 1 s, `lastReadyAgo` after a round that lost the probes to all upstreams is up to 1.2 + 1.2 + 0.5 = 2.9 s, more than witnessFresherBy = 2.35 s. freshnessWait also increases from 1.5 s to 2.5 s, and DeclareSuicide after a lost API comes later.

Do not set `--fail-step-window=0`. Without fail-step, the next round after a round that lost the probes to all upstreams starts up to proxyMaxInterval later, and `lastReadyAgo` is up to 1.2 + 1.2 + 0.5 = 2.9 s, more than witnessFresherBy = 2.35 s.

freshnessWait also counts on the fail-step rounds. Thus the fail-step episode must last until controlPlaneLostAfter + 0.2 s + pollBudget + freshnessWait of each profile: 23.1 s for `slow` with freshnessWait = 1.8 s. The window of 30 s covers it.

The proxy README tells the same for `--fail-step-interval`.
