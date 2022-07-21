# Patches

## Scrape timestamp align

There is a bug in Go runtime. Because of it, tickers from the std library are not precise.
This patch adds a flag to ignore timestamp difference for 10 ms instead of 2 ms,
which promises us a 30% reduction in resource consumption.

https://github.com/prometheus/prometheus/pull/9283

We will not open a PR to Prometheus operator, because this flag is experimental at its current state.
