# CPUModel type Discovery

## Problem

The first approach was to use host-model with a common set of features. This was a mistake, as
libvirt resolves the host-model to the specific host CPU model, which can be different on different nodes
and migration not works.

The second approach was to use "Empty" model in cpu-map directory. It works partially for some CPU combinations.
Other combinations lead to migration problems. These combinations are unpredictable, so no workaround.
The error might be a bug in libvirt when it compares features after resolving the target CPU model (still
need to investigate).

The current approach is to use kvm64 model for Discovery and Features types. This model contains a small
set of features and migration works well.

## Solution

1. Use kvm64 model for Discovery and Features vmclass types.
2. Add patch for kubevirt to prevent adding nodeSelector for cpu model "kvm64".
