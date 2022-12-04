## Patches

### Kafka PEM TLS

Vector's documentation states that it is possible to use either a path to a certificate or a PEM encoded string.
It is true for the most of sources/sinks, but not for Kafka. This patch fixes the logic to be in line with other parts of Vector.

Upstream PR - https://github.com/vectordotdev/vector/pull/15448
