# Patches

## Liveness probe

The `LivenessProbe` was removed in [PR#3502](https://github.com/prometheus-operator/prometheus-operator/pull/3502) until the `StartupProbe` is not implemented. But we didn't face any issues with `LivenessProbe`, so we reverting it back.

## Scrape params

With that patch, we are avoiding `ScrapeTimeout` being greater than `ScrapeInterval`.
