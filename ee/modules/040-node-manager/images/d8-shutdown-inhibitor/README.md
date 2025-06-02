# Deckhouse shutdown inhibitor

## Why?

We need to delay system shutdown until Pods with label are gone from the Node.

## Implementation

d8-shutdown-inhibitor runs as daemon which lock system shutdown with systemd inhibitors, receives shutdown event and wait until Pods are gone.

## Development

Testing and debugging may require reboot, so better use cluster made from
virtual machines, e.g. nested cluster on DVP.

Copy .env.example to .env and change variables to access your node which you use for tests.

Use `task` utility to build and deploy.

`task build` builds binary.
`task deploy` builds binary and transfers to selected Node.

### Useful commands

```shell
systemd-inhibit --list
  List all inhibitor locks.
  
systemd-analyze cat-config systemd/logind.conf
  Check logind configuration, e.g. get InhibitDelayMaxSec.
  
systemctl poweroff --check-inhibitors=yes
systemctl reboot --check-inhibitors=yes
  Use these commands to shutdown/reboot the Node.
```
