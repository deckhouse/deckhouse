## Patches

### kubelet-graceful-shutdown-wait-for-external-inhibitors

This patch supports postponing all pods termination until Node status contains 
a condition with type "GracefulShutdownPostpone" and status "True".

This condition may be set by an external component to prevent Node shutdown
for some scenarios. For example, d8-shutdown-inhibitor.service monitors
if there are Pods with the label "pod.deckhouse.io/inhibit-node-shutdown"
on the Node and prevent shutdown until user migrates these Pods from the Node.

### kubelet-shutdown-events-without-logind

This patch lets kubelet learn that the node is going down on a node that has no
systemd-logind.

Upstream graceful node shutdown is built on logind: kubelet takes a delay
inhibit lock and waits for the PrepareForShutdown signal over D-Bus. Deckhouse
Engine nodes run a minimal systemd with neither logind nor D-Bus — logins are
not supported there — so the shutdown manager cannot start at all, and
shutdownGracePeriodByPodPriority is silently inert.

On those nodes the node agent (nodelet) takes logind's place. It is what sees
the power button, Ctrl+Alt+Del and SIGPWR, and what finally signals PID 1, so
it hands out the same two things over a unix socket: a lock, which is the open
connection itself (releasing it is closing it, exactly as with a logind fd),
and one line of JSON standing in for PrepareForShutdown.

The patch adds one file implementing the existing dbusInhibiter interface
against that socket, and makes the systemDbus factory prefer it when
/run/nodelet/shutdown.sock exists. Everything else is untouched — including the
GracefulShutdownPostpone wait added by the patch above, which is what actually
decides when the pods may be killed. A node with no such socket goes to logind
as before, so nothing changes for DKP nodes.
