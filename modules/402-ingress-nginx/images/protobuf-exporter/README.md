## protobuf_exporter

Receives messages in protobuf format, parses, saves metrics in internal storage, and exports them in Prometheus format.

### Mappings

Mappings are described templates for metrics. Message mappings are made based on indexes.

* `type` — metric type (Histogram, Counter or Gauge).
* `name` — metric name.
* `help` — metric description.
* `ttl` — timeout for storing the metric (if there are no new entries, the metric will be deleted by the timeout). There is no timeout when specifying `0`.
* `labels` — an array of keys for metric labels.
* `bucket` — an array of buckets for Histogram metrics (required for conversion to Prometheus format).

### Message types

1. `CounterMessage` — message for calculating counters.
   * `MappingIndex` — index of the mapping to which this metric belongs.
   * `Labels` — values for metric labels.
   * `Value` — metric value in `uint64` format.
1. `GaugeMessage` — message for calculating gauges.
   * `MappingIndex` — index of the mapping to which this metric belongs.
   * `Labels` — values for metric labels.
   * `Value` — metric value in `float64` format.
1. `HistogramMessage` — message for calculating histograms. The key difference from the standard prometheus_client format is that interpolated values are taken instead of passing a value to calculate the histogram. This reduces the load on IO operations and CPU by pre-aggregating the data on the client-side.
   * `MappingIndex` — index of the mapping to which this metric belongs.
   * `Labels` — values for metric labels.
   * `Sum` — sum of metric values in `float64` format.
   * `Count` — total number of received metrics in `uint64` format.
   * `Buckets` — map of the distribution of values by buckets in the format `map[string]uint64`. Unlike Prometheus, the value should be marked only in the first bucket it got into. It also helps to reduce the amount of data being transmitted.

### Protocol

The message consists of three parts:

1. The first byte is a marker of the protobuf message type.
   * `1` — HistogramMessage
   * `2` — GaugeMessage
   * `3` — CounterMessage
1. The length of the message encoded as uint64 bytes.
1. A message in protobuf format.
