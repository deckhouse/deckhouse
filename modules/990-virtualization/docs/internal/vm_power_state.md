# VM power state

## Reboot differences with kubevirt

Kubevirt has 2 types of reboot:
1. In-Pod reboot: restart VM without exiting from Qemu process.
2. External reboot: delete Kubevirt VirtualMachineInstance and create a new one.

Deckhouse Virtualization promote the idea that reboot issued from inside the VM
is equal to reboot issued externally, e.g. with VirtualMachineOperation.

The only possible restart in Deckhouse Virtualization is to delete VirtualMachineInstance
and create a new one with all possible changes made to VirtualMachine spec.

In-Pod reboot is disabled with some additions to virt-launcher image:
1. Qemu event handler on_restart is set to shutdown to exit from qemu process when reboot is issued.
2. Monitor qemu SHUTDOWN events and write them to /dev/termination-log to catch them later and
   distinguish between guest-rest and guest-shutdown.
These changes are made in images/virt-launcher/scripts/domain-monitor.sh.

## A relationship between runPolicy and runStrategy

Deckhouse Virtualization has 4 run policies:

- AlwaysOff - The system is asked to ensure that no VM is running. This is achieved by stopping
  any VirtualMachineInstance that is associated ith the VM. If a guest is already running,
  it will be stopped.
- AlwaysOn - VM will start immediately after the stop. A stopped VM is scheduled to start when runPolicy changed to AlwaysOn.
- Manual - The system will not automatically turn the VM on or off, instead the user manually controls the VM status by creating VirtualMachineOperation or by issuing reboot or poweroff commands inside the VM.
- AlwaysOnUntilStoppedManually - Similar to Always, except that the VM is only restarted if it terminated
  in an uncontrolled way (e.g. crash) and due to an infrastructure reason (i.e. the node crashed,
  the KVM related process OOMed). This allows a user to determine when the VM should be shut down by
  initiating the shut down inside the guest or creating a VirtualMachineOperation.
  Note: Guest sided crashes (i.e. BSOD) are not covered by this. In such cases liveness checks or the use of a watchdog can help.

AlwaysOff policy is implemented with kubevirt's `runStrategy: Halted`.

AlwaysOn policy is implemented with kubevirt's `runStrategy: Always`

Manual policy is implemented with kubevirt's `runStrategy: Manual` with addition of VM start on guest-reset event.

AlwaysOnUntilStoppedManually policy is implemented with kubevirt's `runStrategy: Manual` with addition of VM start on guest-reset event and stoping VM on failures.

