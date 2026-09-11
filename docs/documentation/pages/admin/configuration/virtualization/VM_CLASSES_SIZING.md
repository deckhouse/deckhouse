---
title: "Virtual machine sizing policy"
permalink: en/admin/configuration/virtualization/vm-classes-sizing.html
description: "The VirtualMachineClass sizing policy: allowed combinations of core count, memory size, and core fraction for project virtual machines."
search: sizing policy, sizingPolicy, coreFraction, memory per core
---

The [`.spec.sizingPolicies`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-sizingpolicies) block defines which combinations of cores, core fraction, and memory are allowed for virtual machines of this class.

{% alert level="warning" %}
Changes to the [`.spec.sizingPolicies`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-sizingpolicies) block affect existing virtual machines.
For virtual machines that no longer meet the new requirements, the `SizingPolicyMatched` condition in the [`.status.conditions`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha2-status-conditions) block gets the `False` status.

When defining policies, take the [CPU topology](../../../user/virtualization/vm-resources.html#cpu-topologies) of virtual machines into account.
{% endalert %}

A policy consists of a list of rules, each applying to its own range of cores. The range is set in the required `cores` block, and ranges of different rules must not overlap, otherwise DP rejects the class.

A valid structure, where the ranges follow one another without overlapping:

```yaml
- cores:
    min: 1
    max: 4
  # ...
- cores:
    min: 5 # The next range starts one above the previous max.
    max: 8
```

An invalid structure, where the value `4` falls into two ranges at once:

```yaml
- cores:
    min: 1
    max: 4
  # ...
- cores:
    min: 4
    max: 8
```

DP doesn't forbid gaps between ranges, but a virtual machine whose number of cores falls outside every range is left without a policy. For this reason, start each range with the value that follows the `max` of the previous one.

Within a range, you set the requirements for memory and for the core fraction:

- `memory`: The minimum and maximum amount of memory. Set either for the whole range or per core, in the nested `memory.perCore` block.
- `coreFractions`: The list of allowed core fractions, for example `[25, 50, 100]` for 25%, 50%, and 100%. If the project owner sets the `coreFraction` parameter of a virtual machine explicitly, the value must come from this list.
- `defaultCoreFraction`: The core fraction that a virtual machine gets if `coreFraction` isn't set in it. The value must be in the `coreFractions` list. If the parameter isn't specified, 100% applies.

A rule with neither `memory` nor `coreFractions` doesn't limit anything, so set at least one of them.

In commercial DP editions, you can set the `defaultCoreFraction` parameter to `Auto`. In that case, the core fraction for VMs without an explicit `coreFraction` is chosen by [vertical autoscaling](../../../user/virtualization/vm-resources.html#automatic-corefraction-auto). `Auto` is a mode, not a share of a core, so it must not appear in the `coreFractions` list.

```yaml
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 8
      coreFractions: [10, 25, 50, 100]
      defaultCoreFraction: Auto
```

The `Auto` value is accepted only when both capabilities are available:

- Vertical autoscaling of virtual machines, which is enabled automatically in commercial DP editions when the [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) module is enabled.
- Changing the number of cores and the amount of memory without a restart, which is enabled by the `HotplugCPUAndMemoryWithInPlaceResize` feature in the [`.spec.settings.featureGates`](/modules/virtualization/configuration.html#parameters-featuregates) parameter.

If at least one of them is unavailable, DP rejects the creation of such a class.

A new default applies only to the virtual machines created after the change. An existing machine already carries the previous value in its specification, and the value stays there until the project owner sets another one.

The following examples show how the amount of memory depends on the number of cores:

- The `memory` parameter sets the same limits for the whole range of cores:

  ```yaml
  - cores:
      min: 1
      max: 4
    memory:
      min: 2Gi
      max: 8Gi
  ```

  A virtual machine with any number of cores from 1 to 4 gets from 2 to 8 GiB of memory, and the number of cores doesn't affect these limits.

- The `memory.perCore` parameter sets the limits per core, and the resulting limits are the product of that value and the number of cores:

  ```yaml
  - cores:
      min: 1
      max: 4
    memory:
      perCore:
        min: 1Gi
        max: 2Gi
  ```

  With such a policy, the allowed amount of memory grows with the number of cores:

  - 1 core: From 1 to 2 GiB.
  - 2 cores: From 2 to 4 GiB.
  - 3 cores: From 3 to 6 GiB.
  - 4 cores: From 4 to 8 GiB.

- The `memory.step` parameter limits the allowed memory values to a grid, so that the project owner can't choose arbitrary amounts.

  Together with `memory.min` and `memory.max`, the step is counted from the minimum:

  ```yaml
  - cores:
      min: 1
      max: 4
    memory:
      min: 2Gi
      max: 8Gi
      step: 1Gi
  ```

  Only the values 2, 3, 4, 5, 6, 7, and 8 GiB are allowed; 2.5 or 7.5 GiB can't be set.

  Together with `memory.perCore`, the step is counted from the memory per core, and the resulting value is then multiplied by the number of cores:

  ```yaml
  - cores:
      min: 1
      max: 4
    memory:
      perCore:
        min: 1Gi
        max: 2Gi
      step: 512Mi
  ```

  Per core, 1, 1.5, and 2 GiB are allowed, so the resulting amount depends on the number of cores:

  - 1 core: 1, 1.5, or 2 GiB.
  - 2 cores: 2, 3, or 4 GiB.
  - 3 cores: 3, 4.5, or 6 GiB.
  - 4 cores: 4, 6, or 8 GiB.

An example of a policy that covers ranges from 1 to 248 cores:

```yaml
spec:
  sizingPolicies:
    # For 1-4 cores, from 1 to 8 GiB of memory is available with a 512 MiB step,
    # that is, 1 GiB, 1.5 GiB, 2 GiB, 2.5 GiB, and so on.
    # All core fractions are available.
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      coreFractions: [5, 10, 20, 50, 100]
      defaultCoreFraction: 50 # Default core fraction for the 1-4 core range.
    # For 5-8 cores, from 5 to 16 GiB of memory is available with a 1 GiB step,
    # that is, 5 GiB, 6 GiB, 7 GiB, and so on.
    # Core fractions are limited to three values.
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      coreFractions: [20, 50, 100]
      defaultCoreFraction: 100 # Default core fraction for the 5-8 core range.
    # For 9-16 cores, from 9 to 32 GiB of memory is available with a 1 GiB step.
    # Core fractions are limited to two values.
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      coreFractions: [50, 100]
    # For 17-248 cores, from 1 to 2 GiB of memory is available per core.
    # The core fraction is 100% only.
    - cores:
        min: 17
        max: 248
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      coreFractions: [100]
```

To configure sizing policies in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

1. Click **Add** in the **Resource allocation rules for virtual machines** block.
1. In the **CPU** block, enter `1` in the **Min** field and `4` in the **Max** field.
1. In the **CPU** block, in the **Allow setting core fractions** field, select the values `5%`, `10%`, `20%`, `50%`, `100%` in order.
1. In the **Memory** block, set the toggle to **Amount per 1 core**.
1. In the **Memory** block, enter `1` in the **Min** field and `8` in the **Max** field.
1. In the **Memory** block, enter `1` in the **Discretization step** field.
1. Add other ranges with the **Add** button, if required.
1. Click **Create**.
