# Description
This report presents a comparative performance analysis of an etcd cluster across four configurations:
- 3M = 3 master nodes
- 2M + A = 2 master nodes + arbiter node
- 2M = 2 master nodes
- 2M + witness = 2 master nodes + witness node

Each configuration is evaluated across three benchmark scenarios:
- range = read operations
- txn-mixed = transactional read/write operations
- put = write operations

Each scenario is executed under two load profiles:
- low
	- conns = 4
	- clients = 16
	- rate
		- range - 1000
		- txn-mixed - 300
		- put - 300
- high
	- conns = 16
	- clients = 64
	- rate
		- range - 8000
		- txn-mixed - 1500
		- put - 2500

Common parameters:
- requests = 100 000
- read
	- limit = 100
	- range_consistency = linearizable
- write
	- keyspace = 100 000
	- key_size = 32
	- val_size = 256
- endpoints = all (except witness)

Parameter notes:
- `endpoints` = the etcd client endpoints targeted by the benchmark tool
- `limit` = the maximum number of keys returned by a single `range` request
- `rate` = the client-side request rate limit configured for a benchmark scenario
- `clients` = the number of benchmark client workers generating requests in parallel
- `conns` = the number of gRPC connections opened by the benchmark tool
- `range_consistency` = the consistency mode for read requests; `linearizable` means reads reflect the latest committed cluster state

The `Benchmark results` section is organized as follows:
- a cross-scenario summary for the `high` load profile;
- benchmark outcome tables for each scenario;
- node-level metrics that help explain how load is distributed within each configuration.

# Environment
- Test environment: Yandex Cloud
- Every setup includes 4 worker nodes
- In the `2M + witness` setup, the witness node is placed on a separate worker node
- Tests are executed with the official etcd benchmark [tool](https://github.com/etcd-io/etcd/tree/main/tools/benchmark)
	- The benchmark tool is launched from `master-0`

# Summary

The `2M + witness` configuration appears to be a reasonable compromise when the main goal is to save one full `master` node while preserving acceptable write performance and quorum behavior. In the `put` scenario, it delivered performance very close to `3M` and `2M`, while the witness node itself consumed minimal CPU, network, and disk resources, making its operational cost genuinely low. At the same time, this setup was noticeably weaker than `3M` in `range` and especially in `txn-mixed`: on the high-load profile, the gap was about 16% for `range` and about 44% for `txn-mixed`. This suggests that witness works well as a low-cost quorum helper, but it is not a full replacement for a third regular etcd member from an overall performance perspective. As a result, witness mode is a reasonable choice when the priorities are cost efficiency, fault tolerance, and `write-heavy` workloads. If substantial `read`-heavy or mixed transactional load is expected, `3M` or `2M + A` remain stronger options.

# Benchmark results

Benchmark traffic was sent to all etcd client endpoints except witness. As a result, the benchmark outcome tables below reflect client-visible behavior on data-serving nodes, while the node-level tables help explain how each topology distributes load internally.

## Cross-scenario summary (`high` load)

| Scenario    | 3M                                | 2M + A                            | 2M                                | 2M + witness                      | Quick reading                                                          |
| ----------- | --------------------------------- | --------------------------------- | --------------------------------- | --------------------------------- | ---------------------------------------------------------------------- |
| `range`     | `159.8512 req/s`, `P95 0.7389 s`  | `179.4595 req/s`, `P95 1.0218 s`  | `121.4614 req/s`, `P95 1.1536 s`  | `134.9792 req/s`, `P95 0.9677 s`  | `2M + witness` is better than `2M`, but still behind `3M` and `2M + A` |
| `txn-mixed` | read `105.8019`, write `53.1719`  | read `97.9596`, write `49.2838`   | read `58.3020`, write `29.1152`   | read `59.5288`, write `29.8161`   | `2M + witness` is almost identical to `2M`                             |
| `put`       | `1724.0946 req/s`, `P95 0.0596 s` | `2002.6404 req/s`, `P95 0.0500 s` | `1743.7470 req/s`, `P95 0.0582 s` | `1700.0164 req/s`, `P95 0.0593 s` | `3M`, `2M`, and `2M + witness` are practically equal                   |

## `range`

### Benchmark outcome

| Setup                    | Load | Throughput, req/s | Avg latency, s | P95 latency, s | Total duration, s |
| ------------------------ | ---- | ----------------- | -------------- | -------------- | ----------------- |
| 3M                       | low  | 163.7944          | 0.0972         | 0.1691         | 610.5             |
| 2M + A                   | low  | 157.4340          | 0.1011         | 0.2517         | 635.1             |
| 2M                       | low  | 115.6846          | 0.1379         | 0.2752         | 864.4             |
| 2M + witness             | low  | 134.4054          | 0.1186         | 0.2237         | 744               |
| 3M                       | high | 159.8512          | 0.3999         | 0.7389         | 625.5             |
| 2M + A                   | high | 179.4595          | 0.3562         | 1.0218         | 557.2             |
| 2M                       | high | 121.4614          | 0.5266         | 1.1536         | 823.3             |
| 2M + witness             | high | 134.9792          | 0.4739         | 0.9677         | 740.8             |
| 2M + witness after patch | high | 167.0157          | 0.3829         | 0.7231         | 598.7             |

| Setup                         | Load | Throughput, req/s | Avg latency, s | P95 latency, s | Total duration, s |
| ----------------------------- | ---- | ----------------- | -------------- | -------------- | ----------------- |
| 2M + witness                  | high | 134.9792          | 0.4739         | 0.9677         | 740.8             |
| 2M + witness after raft patch | high | 167.0157          | 0.3829         | 0.7231         | 598.7             |
### Node-level observations

#### Low load

| Node                    | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ----------------------- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0           | 94.99      | 3.66 KiB       | 497.67 KiB      | 246.42 MB/s    | 779.12 kB/s    |
| 3M / master-1           | 46.26      | 977.29 B       | 485.89 KiB      | 433.57 kB/s    | 122.90 MB/s    |
| 3M / master-2           | 47.61      | 102.40 B       | 507.67 KiB      | 403.78 kB/s    | 123.36 MB/s    |
| 2M + A / master-0       | 96.01      | 678.50 KiB     | 582.11 KiB      | 236.27 MB/s    | 1.05 MB/s      |
| 2M + A / master-1       | 45.81      | 521.96 KiB     | 598.47 KiB      | 441.82 kB/s    | 118.74 MB/s    |
| 2M + A / arbiter        | 35.66      | 20.48 B        | 472.44 KiB      | 248.77 kB/s    | 118.38 MB/s    |
| 2M / master-0           | 96.52      | 147.99 KiB     | 484.40 KiB      | 130.25 MB/s    | 871.51 kB/s    |
| 2M / master-1           | 52.47      | 7.90 KiB       | 472.25 KiB      | 376.06 kB/s    | 130.08 MB/s    |
| 2M + witness / master-0 | 96.26      | 16.37 KiB      | 483.61 KiB      | 150.70 MB/s    | 915.13 kB/s    |
| 2M + witness / master-1 | 64.43      | 26.54 KiB      | 489.22 KiB      | 409.61 kB/s    | 150.79 MB/s    |
| 2M + witness / witness  | 11.47      | 8.48 KiB       | 375.72 KiB      | 95.87 kB/s     | 108.88 kB/s    |

#### High load

| Node                          | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ----------------------------- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0                 | 98.78      | 45.03 KiB      | 497.59 KiB      | 240.66 MB/s    | 838.40 kB/s    |
| 3M / master-1                 | 47.57      | 10.37 KiB      | 510.71 KiB      | 456.23 kB/s    | 120.31 MB/s    |
| 3M / master-2                 | 53.55      | 7.95 KiB       | 536.37 KiB      | 438.18 kB/s    | 120.88 MB/s    |
| 2M + A / master-0             | 99.32      | 115.99 KiB     | 531.65 KiB      | 267.85 MB/s    | 1.01 MB/s      |
| 2M + A / master-1             | 49.33      | 3.47 KiB       | 469.31 KiB      | 401.00 kB/s    | 134.24 MB/s    |
| 2M + A / arbiter              | 37.03      | 1.21 KiB       | 375.36 KiB      | 229.72 kB/s    | 134.79 MB/s    |
| 2M / master-0                 | 98.47      | 346.58 KiB     | 532.13 KiB      | 136.59 MB/s    | 866.38 kB/s    |
| 2M / master-1                 | 48.85      | 250.08 KiB     | 512.54 KiB      | 382.78 kB/s    | 136.52 MB/s    |
| 2M + witness / master-0       | 97.97      | 193.66 KiB     | 560.19 KiB      | 151.48 MB/s    | 958.39 kB/s    |
| 2M + witness / master-1       | 64.63      | 75.06 KiB      | 539.19 KiB      | 429.91 kB/s    | 151.56 MB/s    |
| 2M + witness / witness        | 10.91      | 233.00 B       | 81.84 KiB       | 88.85 kB/s     | 99.83 kB/s     |
| 2M + witness after raft patch | 15.39      |                |                 |                |                |

## `txn-mixed`

### Benchmark outcome

| Setup | Load | Read throughput, req/s | Read avg latency, s | Read P95 latency, s | Write throughput, req/s | Write avg latency, s | Write P95 latency, s | Total duration, s |
| ----- | ---- | ---------------------- | ------------------- | ------------------- | ----------------------- | -------------------- | -------------------- | ----------------- |
| 3M | low | 109.5063 | 0.1122 | 0.2349 | 55.4649 | 0.0567 | 0.1259 | 606.1 |
| 2M + A | low | 90.4706 | 0.1429 | 0.4123 | 45.4324 | 0.0602 | 0.2010 | 735.8 |
| 2M | low | 51.9084 | 0.2460 | 0.5185 | 25.8605 | 0.1189 | 0.2856 | 1285.8 |
| 2M + witness | low | 51.0641 | 0.2493 | 0.5063 | 25.7182 | 0.1211 | 0.2780 | 1302.3 |
| 3M | high | 105.8019 | 0.4802 | 1.2382 | 53.1719 | 0.2450 | 0.6888 | 629 |
| 2M + A | high | 97.9596 | 0.5351 | 1.7915 | 49.2838 | 0.2318 | 0.9217 | 679.1 |
| 2M | high | 58.3020 | 0.8787 | 1.8670 | 29.1152 | 0.4357 | 1.0774 | 1143.9 |
| 2M + witness | high | 59.5288 | 0.8601 | 1.7691 | 29.8161 | 0.4262 | 1.0170 | 1119.2 |

### Node-level observations

#### Low load

| Node | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ---- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0 | 93.03 | 7.59 KiB | 863.95 KiB | 137.93 MB/s | 811.39 kB/s |
| 3M / master-1 | 46.88 | 3.09 KiB | 896.83 KiB | 447.69 kB/s | 69.48 MB/s |
| 3M / master-2 | 49.98 | 92.92 KiB | 874.49 KiB | 417.12 kB/s | 69.18 MB/s |
| 2M + A / master-0 | 94.35 | 50.44 KiB | 819.89 KiB | 105.02 MB/s | 1.01 MB/s |
| 2M + A / master-1 | 56.83 | 7.13 KiB | 850.02 KiB | 416.44 kB/s | 52.69 MB/s |
| 2M + A / arbiter | 44.82 | 0.00 B | 756.86 KiB | 216.64 kB/s | 52.82 MB/s |
| 2M / master-0 | 91.43 | 26.39 KiB | 683.51 KiB | 45.44 MB/s | 856.84 kB/s |
| 2M / master-1 | 63.02 | 7.29 KiB | 687.10 KiB | 363.90 kB/s | 45.47 MB/s |
| 2M + witness / master-0 | 91.38 | 21.72 KiB | 695.21 KiB | 44.72 MB/s | 880.51 kB/s |
| 2M + witness / master-1 | 67.38 | 12.77 KiB | 698.22 KiB | 377.06 kB/s | 44.80 MB/s |
| 2M + witness / witness | 13.91 | 5.94 KiB | 70.06 KiB | 92.79 kB/s | 103.50 kB/s |

#### High load

| Node | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ---- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0 | 96.55 | 55.61 KiB | 686.55 KiB | 132.49 MB/s | 770.85 kB/s |
| 3M / master-1 | 53.76 | 1.23 KiB | 810.45 KiB | 425.56 kB/s | 66.17 MB/s |
| 3M / master-2 | 57.04 | 471.04 B | 804.39 KiB | 397.79 kB/s | 67.07 MB/s |
| 2M + A / master-0 | 97.84 | 29.34 KiB | 680.47 KiB | 114.04 MB/s | 918.88 kB/s |
| 2M + A / master-1 | 61.29 | 3.33 KiB | 824.45 KiB | 384.98 kB/s | 57.14 MB/s |
| 2M + A / arbiter | 46.87 | 0.00 B | 742.02 KiB | 194.41 kB/s | 56.83 MB/s |
| 2M / master-0 | 96.64 | 27.19 KiB | 620.29 KiB | 51.11 MB/s | 812.67 kB/s |
| 2M / master-1 | 74.01 | 17.42 KiB | 649.01 KiB | 356.14 kB/s | 51.14 MB/s |
| 2M + witness / master-0 | 97.28 | 17.34 KiB | 630.85 KiB | 51.99 MB/s | 841.38 kB/s |
| 2M + witness / master-1 | 75.56 | 63.59 KiB | 639.11 KiB | 382.32 kB/s | 52.02 MB/s |
| 2M + witness / witness | 10.71 | 0.00 B | 71.17 KiB | 83.09 kB/s | 98.51 kB/s |

## `put`

### Benchmark outcome

| Setup | Load | Throughput, req/s | Avg latency, s | P95 latency, s | Peak proposal apply rate | Total duration, s |
| ----- | ---- | ----------------- | -------------- | -------------- | ------------------------ | ----------------- |
| 3M | low | 298.9731 | 0.0242 | 0.0430 | 944 | 334.4 |
| 2M + A | low | 299.4304 | 0.0134 | 0.0299 | 623 | 333.9 |
| 2M | low | 298.6038 | 0.0196 | 0.0400 | 621 | 334.8 |
| 2M + witness | low | 299.0317 | 0.0183 | 0.0365 | 623 | 334.4 |
| 3M | high | 1724.0946 | 0.0342 | 0.0596 | 1710 | 58 |
| 2M + A | high | 2002.6404 | 0.0273 | 0.0500 |  | 49.9 |
| 2M | high | 1743.7470 | 0.0330 | 0.0582 | 1120 | 57.3 |
| 2M + witness | high | 1700.0164 | 0.0341 | 0.0593 |  | 58.8 |

### Node-level observations

#### Low load

| Node | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ---- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0 | 51.86 | 436.91 B | 2.15 MiB | 598.68 kB/s | 791.01 kB/s |
| 3M / master-1 | 43.35 | 13.65 B | 2.34 MiB | 670.17 kB/s | 758.71 kB/s |
| 3M / master-2 | 47.71 | 0.00 B | 2.22 MiB | 561.12 kB/s | 1.07 MB/s |
| 2M + A / master-0 | 52.63 | 7.37 KiB | 2.28 MiB | 485.51 kB/s | 1.06 MB/s |
| 2M + A / master-1 | 37.96 | 1010.35 B | 2.36 MiB | 535.46 kB/s | 395.55 kB/s |
| 2M + A / arbiter | 31.17 | 0.00 B | 2.29 MiB | 387.38 kB/s | 536.01 kB/s |
| 2M / master-0 | 58.62 | 7.41 KiB | 2.18 MiB | 405.65 kB/s | 1.06 MB/s |
| 2M / master-1 | 40.42 | 10.19 KiB | 2.19 MiB | 561.28 kB/s | 459.11 kB/s |
| 2M + witness / master-0 | 53.55 | 1.23 KiB | 2.23 MiB | 403.70 kB/s | 873.10 kB/s |
| 2M + witness / master-1 | 41.19 | 2.24 KiB | 2.04 MiB | 593.63 kB/s | 732.07 kB/s |
| 2M + witness / witness | 11.02 | 0.00 B | 69.37 KiB | 107.62 kB/s | 106.35 kB/s |

#### High load

| Node | Avg CPU, % | Avg read bytes | Avg write bytes | Avg RX network | Avg TX network |
| ---- | ---------- | -------------- | --------------- | -------------- | -------------- |
| 3M / master-0 | 55.44 | 0.00 B | 4.09 MiB | 1.58 MB/s | 1.55 MB/s |
| 3M / master-1 | 38.36 | 0.00 B | 4.89 MiB | 1.32 MB/s | 2.04 MB/s |
| 3M / master-2 | 43.68 | 0.00 B | 4.17 MiB | 1.42 MB/s | 1.59 MB/s |
| 2M + A / master-0 | 45.11 | 19.87 KiB | 4.09 MiB | 1.66 MB/s | 1.89 MB/s |
| 2M + A / master-1 | 33.66 | 0.00 B | 4.44 MiB | 1.49 MB/s | 867.46 kB/s |
| 2M + A / arbiter | 26.54 | 0.00 B | 4.53 MiB | 1.12 MB/s | 2.04 MB/s |
| 2M / master-0 | 56.63 | 35.73 KiB | 3.87 MiB | 998.96 kB/s | 2.04 MB/s |
| 2M / master-1 | 34.69 | 136.53 B | 4.09 MiB | 1.58 MB/s | 1.08 MB/s |
| 2M + witness / master-0 | 55.17 | 0.00 B | 3.72 MiB | 989.19 kB/s | 1.83 MB/s |
| 2M + witness / master-1 | 35.91 | 2.53 KiB | 3.72 MiB | 1.49 MB/s | 1.24 MB/s |
| 2M + witness / witness | 13.52 | 0.00 B | 125.47 KiB | 66.36 kB/s | 103.98 kB/s |
