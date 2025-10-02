# Number of CPU cores

## Logic for calculating the number of sockets and cores

For .spec.cpu.cores <= 16:

- One socket is created with the number of cores equal to the specified value.
- Core increment step: 1
- Allowed values: any number from 1 to 16 inclusive.

For 16 < .spec.cpu.cores <= 32:

- Two sockets are created with the same number of cores in each.
- Core increment step: 2
- Allowed values: 18, 20, 22, ..., 32.
- Minimum cores per socket: 9
- Maximum cores per socket: 16

For 32 < .spec.cpu.cores <= 64:

- Four sockets are created with the same number of cores in each.
- Core increment step: 4
- Allowed values: 36, 40, 44, ..., 64.
- Minimum cores per socket: 9
- Maximum cores per socket: 16

For .spec.cpu.cores > 64:

- Eight sockets are created with the same number of cores in each.
- Core increment step: 8
- Allowed values: 72, 80, ...
- Minimum cores per socket: 8

## Value validation

Validation of .spec.cpu.cores values is performed via a webhook.

## Displaying VM topology

The current VM topology (actual number of sockets and cores) is displayed in the VM status in the following format:

```yaml
status:
  resources:
    cpu:
      coreFraction: 100%
      cores: 18
      requestedCores: "18"
      runtimeOverhead: "0"
      topology:
        sockets: 2
        coresPerSocket: 9
```
