---
title: Pod and container security settings
description: "Overview of pod and container security settings: managing privileges, permissions, system calls, and node resource access."
permalink: user/security/pod-settings.html
lang: en
---

There are various parameters in the pod manifests that directly affect the security of containers.

The majority of these parameters are configured in the `securityContext` and `containers[].securityContext` sections.

This page covers the main parameters used to configure pod and container security:

- [`runAsUser`](#runasuser)
- [`runAsNonRoot`](#runasnonroot)
- [`runAsGroup`](#runasgroup)
- [`readOnlyRootFilesystem`](#readonlyrootfilesystem)
- [`fsGroup`](#fsgroup)
- [`fsGroupChangePolicy`](#fsgroupchangepolicy)
- [`appArmorProfile`](#apparmorprofile)
- [`seccompProfile`](#seccompprofile)
- [`capabilities`](#capabilities) (`add`/`drop`)
- [`allowPrivilegeEscalation`](#allowprivilegeescalation)
- [`privileged`](#privileged)
- [`procMount`](#procmount)
- [`sysctls`](#sysctls)
- [`hostNetwork`](#hostnetwork)
- [`hostPID`](#hostpid)
- [`hostIPC`](#hostipc)
- [`hostPath`](#hostpath)
- [`automountServiceAccountToken`](#automountserviceaccounttoken)

{% alert level="info" %}
In Deckhouse Kubernetes Platform (DKP), the [`admission-policy-engine`](/modules/admission-policy-engine/) module is responsible for monitoring the allowed values specified in these parameters.
{% endalert %}

## runAsUser

The `runAsUser` parameter specifies a digital user ID (`UID`) under which all processes within the container will run.

It directly controls the permissions of processes in Linux. Using this option, you can forcefully remove superuser (`root`) rights from the application at the kernel level, even if the Docker image itself is launched with UID `0`.

In the Linux kernel, file system and process security is based on digital identifiers (UID) rather than text names. The `root` superuser always has UID `0`. By default, if the `USER` instruction is not specified in the `Dockerfile`, the container runtime starts the application with UID `0`.

For example, if a container runs a web server as `root` and an attacker finds a vulnerability (for example, `Remote Code Execution`), he gains full control of the sandbox with UID `0`. This makes it much easier to attack the host operating system.

When using `runAsUser`, Kubernetes passes the specified UID to the container runtime when creating a process. In this case:

- **Image settings are ignored**: Text or numeric users specified by the developer in `Dockerfile` (via the `USER` instruction) are completely overridden by the value from the manifest.
- **Process rights change**: All processes created by the container receive the specified UID (for example, `10001`). From this point on, they are subject to standard Linux access control rules for ordinary users.

Running processes with a non-root UID (any number other than `0`) is a fundamental security rule (principle of least privilege). If the application is compromised, an attacker with UID `10001` will not be able to modify system files inside the container, install malicious packages via `apt`/`apk`, or make sensitive system calls to the host kernel.

{% alert level="warning" %}
If you specify a random UID (for example, `2000`), this user inside the container may not have rights to read the files of the application itself if they were copied into the image with `root`:`root` rights. Developers need to prepare Docker images in advance (by using `CHOWN` for working directories) so that the application can successfully run under a non-root user.
{% endalert %}

### Parameter location

The parameter can be set for the entire pod at once (the settings will be inherited by all containers), or individually for a specific container. The value set for a container has a higher priority.

The parameter can be set in one of the following fields:

* `spec.securityContext.runAsUser`: For the entire pod.
* `spec.containers[].securityContext.runAsUser`: For a specific container.

### Available parameter values

The parameter accepts a positive integer (`integer`), which is a UID in Linux.
The following options are available:

* `0`: Run as `root` (strongly not recommended).
* `1000` and higher (up to `65535` or `4294967295`, depending on the architecture): Run as a regular user (it is recommended to select random high IDs, for example `10001`, to avoid matches with system UIDs on the host).

### Configuration example

The following is a configuration example for forcing the container to run as a non-root user with UID `10001`:

```yaml
spec:
  containers:
  - name: secure-web-app
    image: my-app:latest
    securityContext:
      runAsUser: 10001
```

## runAsNonRoot

The `runAsNonRoot` parameter determines whether the container must be launched exclusively as a non-root user (UID other than `0`).

It enables a built-in validation performed by the `kubelet` agent. Using this parameter, Kubernetes checks the manifest and the Docker image itself before starting, completely blocking the launch of the container if the resulting process UID is `0`.

Typically, Kubernetes trusts the Docker image metadata. If the developer forgot to specify a non-root user in the `Dockerfile`, the container will run as `root` (UID `0`). The `runAsNonRoot` parameter makes `kubelet` perform a strict inspection before starting the container.

For example, if the `runAsNonRoot: true` flag is set in the pod manifest, `kubelet` requests information about the user from the image from the container runtime. If USER `root` is configured there or there is no this instruction at all (which means UID `0`), Kubernetes aborts the startup and puts the pod into an error state.

When using `runAsNonRoot: true`, The `kubelet` agent parses the resulting UID with which the process should start. In this case:

- **A bunch of parameters are checked**: If `runAsUser: 0` is specified in the manifest, Kubernetes will immediately block the pod.
- **The image manifest is parsed**: If `securityContext` does not specify a specific `runAsUser`, the `kubelet` agent looks at the UID inside the Docker image itself. If `0` (or the text name `root`) is detected there, the pod will not start.
- **A startup error is displayed**: Instead of a running insecure container, you will receive the status `CreateContainerConfigError` or `ContainerCannotRun` with the following description: `Container has runAsNonRoot and image will run as root`.

This parameter serves as a "safety cushion" and the main line of defense for the cluster from the human factor. It guarantees that a container running with superuser rights cannot physically enter the production environment. This is critically important, since compromising the `root` container opens a direct path for an attacker to exploit vulnerabilities in the host kernel and potentially "escape from the container".

{% alert level="warning" %}
Some Docker images use text usernames instead of digital UIDs (for example, USER `nginx`). If the image does not have a name/ID mapping table (`/etc/passwd`) configured, `kubelet` will not be able to determine the numeric UID during provisioning and will lock the container even if that user is not `root`. To avoid this problem, use `runAsNonRoot: true` while also explicitly providing a digital ID through `runAsUser`.
{% endalert %}

### Parameter location

The parameter can be set for the entire pod at once (the settings will be inherited by all containers), or individually for a specific container:

* `spec.securityContext.runAsNonRoot`: For the entire pod.
* `spec.containers[].securityContext.runAsNonRoot`: For a specific container.

### Available parameter values

The parameter is of boolean type.
The following values ​​are available:

* `false`: Check for non-root user is disabled (the default value).
* `true`: Running as `root` is strictly prohibited (recommended for all applications).

### Configuration example

The following is a configuration example for disabling running as `root` and forces a secure UID:

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 10001
  containers:
  - name: protected-app
    image: my-app:latest
```

## runAsGroup

The `runAsGroup` parameter specifies a numeric identifier of the main group (GID) under which all processes within the container will be launched.

It complements the `runAsUser` parameter and controls group access rights of processes to Linux files and devices. Using it, you can flexibly differentiate shared access rights for several containers or configure secure interaction with local drives.

In Linux, access to files is distributed at three levels: for the owner (`User`), for members of his main group (`Group`) and for everyone else (`Others`). Groups are identified by the numeric GID, where `0` is the superuser group `root`. By default, if the parameter is not specified, container runtime assigns processes the GID `0` or the group that is hard-coded for the user in the `/etc/passwd` file inside the Docker image.

For example, if an application creates logs or temporary files, they are written with the group `root` (GID `0`) by default. If another, neighboring container (for example, a log collection agent) needs to give access to the same directory, engineers have to give the files excessive `chmod 666` rights (read and write for everyone on the system), which violates security rules.

When using `runAsGroup`, Kubernetes passes the specified GID to the container runtime, overriding the operating system's default behavior. In this case:

- **Primary group is assigned**: The container's primary process and all its child processes are forced to the specified GID (for example, `20002`).
- **New files are marked**: All new files, directories or sockets that the container creates during its operation will automatically receive this GID as its owning group.

Fixing a non-root GID (other than `0`) prevents container processes from accidentally or intentionally accessing protected system files of the `root` group. In addition, this allows you to safely organize the joint operation of several containers with common data (via `emptyDir` or PersistentVolume) without issuing dangerous global read and write rights to extraneous processes.

{% alert level="warning" %}
Just as in the case of `runAsUser`, specifying a random GID can lead to `Permission denied` errors at startup if the application binaries inside the Docker image are owned exclusively by the `root` group and are not readable by others. The image must be designed taking into account that the application will run under a non-standard group.
{% endalert %}

### Parameter location

The parameter can be set for the entire pod at once (the settings will be inherited by all containers), or individually for a specific container:

* `spec.securityContext.runAsGroup`: For the entire pod.
* `spec.containers[].securityContext.runAsGroup`: For a specific container.

### Available parameter values

The parameter accepts a positive integer (`integer`), which is GID on Linux.
The following options are available:

* `0`: Assigning the main group `root` (not recommended).
* `1000` and higher (up to `4294967295`): Assigning a non-root group (it is recommended to select high values, for example `10002`, consistent with your rights delimitation policy).

### Configuration example

The following is a pod configuration example where processes run under a non-root user and are part of a dedicated safe group:

```yaml
spec:
  securityContext:
    runAsUser: 10001
    runAsGroup: 10002
  containers:
  - name: shared-data-app
    image: my-app:latest
```

## readOnlyRootFilesystem

The `readOnlyRootFilesystem` setting controls whether processes inside a container are allowed to modify or create files on its own underlying disk.

It brings the immutability of containers to the hardware level of the Linux kernel. Using this option, you can block any attempts to write to the container's file system, turning its image into a static and protected environment.

Containers are created based on Docker image layers, on top of which runtime is superimposed by one top thin layer with write permissions (`Read-Write layer`). By default, an application is free to create files in `/tmp`, install utilities, or overwrite its own configuration files.

For example, if an attacker hacks a web application, their typical first step is to download the malicious script or backdoor to a temporary directory (for example, `/tmp/malware.sh`), give it execution rights, and run it. If the file system is writable, the Linux kernel will allow it to do so.

When using `readOnlyRootFilesystem: true`, Kubernetes instructs the container runtime to mount the container root layer (`/`) in ro mode (`read-only`). In this case:

- **Writing to root is blocked**: Any system attempt to create, change or delete a file in any container directory (if it is not a specially mounted external drive) will be stopped by the kernel.
- **A system error is triggered**: An application that tries to write a log or temporary file to a basic disk will immediately receive an operating system failure with a `Read-only file system` error.

This parameter implements the concept of immutable infrastructure and eliminates the risk of a hacker getting a long-term hold on the container. Even if a critical vulnerability (`RCE`) is found in the application, the attacker will not be physically able to save malicious files, replace binaries or modify the code. This is one of the most effective defense practices against modern automated attacks.

{% alert level="warning" %}
Most modern applications (as well as system libraries inside the image) cannot start if they are completely prohibited from writing temporary data (for example, PID files or logs). To prevent the application from crashing with an error at startup, all paths necessary for recording (such as `/tmp`, `/var/run`, `/cache`) must be point-mounted as temporary RAM disks using the `emptyDir` mechanisms directly in the Kubernetes manifest.
{% endalert %}

### Parameter location

The parameter is set exclusively at the level of a specific container in the `spec.containers[].securityContext.readOnlyRootFilesystem` field.

### Available parameter values

The parameter is of boolean type.
The following values ​​are available:

- `false`: The container's file system is writable (default value).
- `true`: The `root` file system is write-locked (recommended for maximum security).

### Configuration example

The following is a configuration example switching the container to read-only mode with safe allocation of the temporary `/tmp` folder for legitimate recording:

```yaml
spec:
  containers:
  - name: immutable-api
    image: my-app:latest
    securityContext:
      readOnlyRootFilesystem: true
    volumeMounts:
    - mountPath: /tmp
      name: temp-storage
  volumes:
  - name: temp-storage
    emptyDir: {}
```

## fsGroup

The `fsGroup` parameter defines a numeric identifier of the Linux group (GID) to which all PersistentVolumes mounted to the pod will be forced to belong.

It manages access rights to external drives at the host and container file system level. With it, Kubernetes automatically resolves the permissions compatibility issue by allowing non-root processes to seamlessly read and write data to mounted storage without the need for excessive administrative rights.

When a PersistentVolume is mounted to the pod, the files on it retain the UID and GID with which they were originally written (often this is `0:0`, that is, `root`). If the container itself is run as a secure non-root user (for example, `runAsUser: 10001`), the Linux kernel will block access to the disk.

For example, a PostgreSQL database is launched in a container under the user `postgres` (UID `999`). When mounting a network drive, the process tries to initialize files in the data directory, but receives a `Permission denied` error because the directory on the drive is owned by `root`. Engineers have to manually change disk permissions, which is inconvenient and unsafe.

If using `fsGroup`, when a pod starts, Kubernetes performs a special procedure to prepare volumes before starting the main containers. In this case:

- **Ownership on the disk is changed**: Kubernetes automatically performs an operation similar to `chown`, making the specified `fsGroup` (for example, `30003`) the group owner of all files and directories on the mounted disk.
- **The SGID flag is set**: A special `set-group-ID` flag is applied to the root directory of the volume. Thanks to this, all new files that the container creates on this disk during operation will automatically inherit the GID from `fsGroup`, and not the main group of the container.
- **Rights are added to processes**: The ID `fsGroup` is added to the list of supplemental groups for each container in the pod, giving processes legitimate access to volume files.

This parameter eliminates the need to run containers as `root` just for them to have access to their disks. It also eliminates the dangerous practice of setting `chmod 777` permissions on network-attached storage, isolating application-specific data at the level of a dedicated Linux group.

{% alert level="warning" %}
By default, every time a pod is restarted, Kubernetes recursively traverses all files on the mounted disk to check and change their GID. If there are numerous small files stored on the database disk, this process can take a lot of time, causing the pod to hang in the `ContainerCreating` status. To correct this behavior, use `fsGroup` together with the `fsGroupChangePolicy: OnRootMismatch` option.
{% endalert %}

### Parameter location

The setting is set in the `spec.securityContext.fsGroup` field exclusively at the level of the entire pod, since volumes are mounted to the pod as a whole.

### Available parameter values

The parameter accepts a positive integer, which is GID on Linux.
The following options are available:

* `0`: Use the `root` group for volumes (not recommended).
* `1000` and higher (up to `4294967295`): Dedicated group identifier for disk sharing (recommended).

### Configuration example

The following is a configuration example of a pod where a non-root container gets automatic secure access to a mounted persistent volume:

```yaml
spec:
  securityContext:
    runAsUser: 10001
    fsGroup: 30003
  containers:
  - name: db-app
    image: my-db:latest
    volumeMounts:
    - mountPath: /var/lib/data
      name: storage
  volumes:
  - name: storage
    persistentVolumeClaim:
      claimName: db-pvc
```

## supplementalGroups

The `supplementalGroups` parameter specifies a list of additional Linux numeric group IDs (GID) that will be forced to be added to the container process in addition to its main group.

In Linux, each process always has one main group (specified via `runAsGroup`) and a list of supplemental groups. The extra groups mechanism is used by the kernel to grant a user access rights to various shared system resources, shared directories, or physical devices on the host.

The list of supplemental groups is formed by listing numeric identifiers. When a pod is launched, the Linux kernel merges the process's main group and this list, expanding the container's access rights.

The following is a configuration example for a process inside a container:

| Parameter | Value |
| --------- | ----- |
| Primary user, UID (`runAsUser`) | `10001` |
| Primary group, GID (`runAsGroup`) | `10002` |
| Supplemental groups, GID (`supplementalGroups`) | `[40001, 40002]` |

If there are files on a mounted disk or inside a container that belong to the `40001` group, a process with this configuration will be able to read or modify them directly (depending on standard UNIX file permissions) because it is a legitimate member of that group.

### Parameter location

The setting is set in the `spec.securityContext.supplementalGroups` field exclusively at the level of the entire pod, since the list of supplemental groups applies to all containers (including init containers) within the pod.

### Available parameter values

The parameter accepts a list of positive integers (`array` of `integer`) representing the Linux system GID.
The following options are available:

* `0`: Adding the `root` group as an additional one (strongly not recommended).
* `1–999`: Linux system groups (for example, groups for accessing audio devices or logging). Use with caution to prevent giving out unnecessary rights on the node.
* `1000` and higher (up to `4294967295`): User security groups for jointly differentiating access rights to file storages (recommended option).

The `supplementalGroups` parameter allows you to implement a fine-grained division of access rights to shared data (for example, to files in `emptyDir` or PersistentVolume) between different pods without escalating privileges to the `root` level. Instead of issuing redundant read and write permissions to all system users (`chmod 777`), engineers can combine multiple independent services into one common secure group.

{% alert level="warning" %}
Important details:

- Relationship with `fsGroup`. The `fsGroup` parameter also adds the specified GID to the list of additional process groups. The difference is that `fsGroup` additionally changes the owner of files on the mounted disk and sets the `SGID` flag, while `supplementalGroups` only adds rights to the process itself, without modifying the volume file system at startup.
- Ignoring text names. Kubernetes only accepts numeric group ID formats. Text group settings from the `/etc/group` file inside the Docker image are completely ignored.
{% endalert %}

### Configuration example

The following is a configuration example of a pod whose process runs as a non-root user and accesses two different shared stores through supplemental groups `40001` and `40002`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: shared-groups-pod
spec:
  securityContext:
    runAsUser: 10001
    runAsGroup: 10002
    supplementalGroups:
      - 40001 # Group for accessing the shared log storage.
      - 40002 # Group for media file operations.
  containers:
    - name: app
      image: nginx
      volumeMounts:
        - mountPath: /var/log/shared
          name: log-volume
  volumes:
    - name: log-volume
      persistentVolumeClaim:
        claimName: shared-logs-pvc
```

## fsGroupChangePolicy

The `fsGroupChangePolicy` setting determines how Kubernetes checks and changes the ownership (GID) of files on mounted disks when the pod starts.

It directly controls the behavior of the `kubelet` agent during the initialization phase of stores. Using this parameter, you can optimize the startup time of applications that work with large amounts of data, preventing long cluster downtimes.

When a pod is configured with `fsGroup`, Kubernetes must ensure that all files on the mounted volume belong to that group. By default, `kubelet` performs a full recursive check and change of ownership (analogous to the `chown -R` command) for each file on disk every time the pod is started or restarted.

For example, if you are running a database (such as PostgreSQL or Elasticsearch) with a multi-terabyte disk containing numerous small files, the `kubelet` agent's system crawl of the disk may take 15-30 minutes. All this time, the pod will be in the `ContainerCreating` state, blocking the application and violating the availability metrics (uptime).

When configuring `fsGroupChangePolicy`, Kubernetes changes the algorithm for preparing volumes before starting containers. In this case:

- **A standard traversal is enabled** (`Always`): `kubelet` always recursively traverses all files and changes the GID on the fly, regardless of whether the permissions there are correct or not.
- **Smart check is enabled** (`OnRootMismatch`): `kubelet` checks the rights only of the very root directory of the mounted disk. If its GID already matches the one specified in `fsGroup`, the recursive traversal of terabytes of data is skipped entirely and containers start instantly. If the rights do not match (for example, the disk is connected for the first time), then the rights are updated for all attached files.

This parameter addresses the critical issue of infrastructure hanging when scaling, updating pods, or failing over nodes (failover). At the same time, strict data isolation is maintained: access rights are guaranteed to be adjusted if the disk is moved from another environment or was created with default `root` rights.

{% alert level="warning" %}
The `OnRootMismatch` policy is not supported by all types of storage (although it works perfectly with standard PersistentVolumeClaims based on block and network devices, such as AWS EBS, Ceph RBD or local disks). Additionally, if files within a volume were manually created by a third-party process with different GIDs, smart checking against the root folder may not notice hidden discrepancies in the permissions of deeper subdirectories.
{% endalert %}

### Parameter location

The setting is set in the `spec.securityContext.fsGroupChangePolicy` field exclusively at the level of the entire pod, since volumes are mounted to the pod as a whole.

### Available parameter values

The parameter is of string type.
The following values ​​are available:

* `Always`: Always recursively change the rights to all files at each start (default value).
* `OnRootMismatch`: Change rights recursively only if the rights of the volume root directory do not match the `fsGroup` parameter (recommended for databases and large storages).

### Configuration example

The following is an optimized configuration example for quick launch of a pod with a database and a terabyte disk:

```yaml
spec:
  securityContext:
    runAsUser: 10001
    fsGroup: 30003
    fsGroupChangePolicy: OnRootMismatch # Skip recursive check if the volume root is already configured.
  containers:
  - name: heavy-db
    image: my-db:latest
    volumeMounts:
    - mountPath: /data
      name: big-volume
  volumes:
  - name: big-volume
    persistentVolumeClaim:
      claimName: heavy-db-pvc
```

## appArmorProfile

The `appArmorProfile` parameter determines which Mandatory Access Control (MAC) profile will be imposed on the container at the Linux kernel level.

It controls the behavior of the AppArmor system security module running on the cluster host nodes. Using this parameter, you can block specific actions inside the container (for example, reading host configuration files, executing binary files, or writing to system folders), even if the process is running as `root`.

AppArmor is a Linux security subsystem that allows a program to be associated with a security profile that defines its permissions. Unlike standard Linux permissions, AppArmor's restrictions are imposed at the top and cannot be bypassed even by the superuser `root`.

The list of allowed actions is generated by creating a special file, the AppArmor profile on the host node in the `/etc/apparmor.d` directory.

A profile is a text structure that specifies detailed rules for access to the file system, network, and system calls.
Example:

```shell
profile k8s-apparmor-example-deny-write flags=(attach_disconnected) {
  include <abstractions/base>
  
  # Allow reading everything.
  /** r,
  
  # Allow writing everywhere except /etc.
  /** w,
  audit deny /etc/** w,
}
```

There are three types of profiles:

- Built-in runtime profile (`RuntimeDefault`): Contains a basic set of containerization restrictions that are optimal for most tasks.
- Profile without restrictions (`Unconfined`): Completely disables AppArmor protection for the container.
- User profiles (`Localhost`): Custom profiles that the administrator must pre-load into the Linux kernel on each node in the `/etc/apparmor.d` directory.

AppArmor provides a powerful additional layer of isolation and protection against zero-day vulnerabilities. If an attacker has obtained code execution in a container and even elevated his rights to `root`, a strict AppArmor profile will prevent him from changing application configuration files, reading host secrets, or executing dangerous system utilities.

### Parameter location

The parameter is set in the following fields:

* `spec.securityContext.appArmorProfile`: At the pod level.
* `spec.containers[].securityContext.appArmorProfile`: At the container level.

### Available parameter values

The following values are available:

* `RuntimeDefault`: Default container runtime profile (the recommended option).
* `Unconfined`: No AppArmor restrictions (less secure).
* `Localhost`: A user profile that must be preloaded into the operating system kernel on each cluster node.

  This parameter also specifies the string parameter `localhostProfile` containing the exact profile name registered in the host operating system.

{% alert level="warning" %}
Before the introduction of a designated field in the API, AppArmor profiles were configured through text annotations in the pod metadata. Since Kubernetes version 1.30, this syntax has been deprecated, and in version 1.36, support for annotations was completely removed from Kubernetes code.

The syntax required constructing a composite key containing the exact name of the target container:

- Location: `metadata.annotations["container.apparmor.security.beta.kubernetes.io/<container_name>"]`.
- Values: `runtime/default` (current `RuntimeDefault`), `localhost/<profile_name>` (current `Localhost`), or `unconfined` (current `Unconfined`).

The critical flaw of the legacy format was that the API server did not validate typos in the container name inside the annotation. If the name was misspelled, Kubernetes would still launch the container without any AppArmor protection at all. When updating clusters to versions 1.30+, this format must be forced to be rewritten to the new `securityContext.appArmorProfile`.
{% endalert %}

### Configuration example

The following is a configuration example of a pod using a custom AppArmor profile named `k8s-apparmor-example-deny-write`, which was pre-loaded into the OS on the cluster nodes:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: secure-apparmor-pod
spec:
  containers:
    - name: app
      image: nginx
      securityContext:
        appArmorProfile:
          type: Localhost
          localhostProfile: k8s-apparmor-example-deny-write # Exact profile name loaded in the node OS kernel.
```

## seccompProfile

The `seccompProfile` parameter limits the list of system calls available to processes in the container using a special profile.

A system call (`syscall`) is a call from a user space program to the Linux kernel for a "privileged" action.
The process cannot directly manage devices, memory, network and other system resources. It asks the kernel to do this through a `syscall`.

The list of allowed system calls is formed by creating a special file with the `seccomp` profile.
A profile is a JSON structure in which permissions are specified.
Example:

```json
{
    "defaultAction": "SCMP_ACT_ERRNO",
    "architectures": [
        "SCMP_ARCH_X86_64",
        "SCMP_ARCH_X86",
        "SCMP_ARCH_X32"
    ],
    "syscalls": [
        {
            "names": [
                "read",
                "write",
                "exit",
                "exit_group"
            ],
            "action": "SCMP_ACT_ALLOW"
        }
    ]
}
```

There are three profile types:

- Built-in runtime profile (`RuntimeDefault`): Contains a basic set of allowed system calls that is optimal for most containers.
- Profile without restrictions (`Unconfined`): Allows you to perform any action in the container.
- User profiles.

The fewer system calls allowed, the smaller the attack surface. Even if an attacker has obtained code execution in the container, disabling critical system calls can limit the development of the attack.

### Parameter location

The parameter is set in the following fields:

- `spec.securityContext.seccompProfile`: At the pod level.
- `spec.containers[].securityContext.seccompProfile`: At the container level.

### Available parameter values

Allowed values:

- `RuntimeDefault`: Default container runtime profile (the recommended option).
- `Unconfined`: No `seccomp` restrictions (less secure).
- `Localhost`: User profile located on each node in the `/var/lib/kubelet/seccomp` directory.

  For this parameter, the `localhostProfile` parameter is also specified. It contains the relative path to the profile inside the `/var/lib/kubelet/seccomp` directory.

### Configuration example

The following is a configuration example of a pod using the user profile located in the file `/var/lib/kubelet/seccomp/my-profiles/secure.json`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: secure-pod
spec:
  securityContext:
    seccompProfile:
      type: Localhost
      localhostProfile: my-profiles/secure.json  # Path relative to /var/lib/kubelet/seccomp/.
  containers:
    - name: app
      image: nginx

```

## seLinuxOptions

The `seLinuxOptions` parameter limits the capabilities of processes in the container using Mandatory Access Control (MAC) tags at the Linux kernel level.

SELinux (Security-Enhanced Linux) is a Linux security subsystem that controls access based on policies and security contexts (labels) assigned to processes, files, ports, and devices. Unlike standard Linux permissions, SELinux restrictions are imposed on top, so that processes are isolated from each other, even if they are running as `root`. This mechanism is enabled by default and is standard in the Red Hat, CentOS, Fedora, Rocky Linux and AlmaLinux distributions.

The SELinux security context is a colon-separated string that consists of four main components: `user:role:type:level`.

In Kubernetes, these components are passed to the container runtime through a special parameter structure.
Example component structure:

<!-- markdownlint-disable MD031 -->
```console
User:   system_u     # SELinux system user.
Role:   system_r     # SELinux system role for processes.
Type:   container_t  # Domain that defines container access rules.
Level:  s0:c123,c456 # Categories for multi-category isolation (MCS).
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

In most cases, container runtimes (`containerd`, `CRI-O`) automatically generate random unique categories (levels) for each pod. This prevents two containers with the same `container_t` type from physically accessing each other's files on the host node. The `seLinuxOptions` parameter is used when this standard mechanism needs to be overridden.

SELinux provides strict isolation at the host file system level and interprocess communication. If an attacker compromises an application inside a container, SELinux blocks any attempts to read foreign files on the host (for example, in the `/var/lib/kubelet/pods/` directory), access host sockets, or access devices in `/dev`, preventing "escape from the container".

### Parameter location

The parameter is set in the following fields:

* `spec.securityContext.seLinuxOptions`: At the pod level.
* `spec.containers[].securityContext.seLinuxOptions`: At the container level.

### Available parameter values

The parameter accepts an object consisting of four optional string fields that correspond to SELinux context components:

* `user`: SELinux user (for example, `system_u`).
* `role`: SELinux role (for example, `system_r`).
* `type`: SELinux security type or domain (for example, `container_t` or a specialized domain like `spc_t` for super-privileged containers).
* `level`: Sensitivity level and MCS category (for example, `s0:c100,c200`). Used to explicitly separate access to shared disks.

{% alert level="warning" %}
By default, Kubernetes automatically labels all mounted PersistentVolumes (through `relabeling`), assigning files on disk the same MCS level that the pod starts with. If you need to disable this automatic process (for example, a disk is mounted simultaneously in several pods in `ReadWriteMany` mode and automatic label change breaks access), a special mount point with the `mountOptions: ["context=xyz"]` flag is used in the volume settings (`spec.volumes[].persistentVolumeClaim`).
{% endalert %}

### Configuration example

The following is a configuration example of a pod launched with a strictly fixed SELinux context for working with a specialized data type and shared directory:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: secure-selinux-pod
spec:
  securityContext:
    seLinuxOptions:
      user: "system_u"
      role: "system_r"
      type: "container_t"
      level: "s0:c123,c456" # Explicit MCS categories for access to shared files.
  containers:
    - name: app
      image: nginx
```

## capabilities

The `capabilities` parameter is responsible for managing Linux kernel privileges (Linux Capabilities) for processes inside the container.

Linux Capabilities is a mechanism in the Linux kernel that breaks down the powers of the superuser (`root`) into dozens of separate, independent privileges.
Each privilege has its own text identifier. In total, the Linux kernel has about 40 different capabilities.
Here are the most common of them:

- `NET_BIND_SERVICE`: Allows a process (even not as `root`) to listen on system ports (for example, `80` or `443`).
- `SYS_TIME`: Allows changing the system time on a node.
- `NET_ADMIN`: Allows you to configure network interfaces, routing rules and a firewall within the container network (often used in service mesh, such as Istio).
- `CHOWN`: Allows you to randomly change the owner of files.

Capabilities implement the principle of least privilege. Using it, you can give the user a single specific kernel privilege without giving full `root` access, or take away some privileges from the `root` user.

### Parameter location

The parameter consists of two parts:

- `Drop`: List of privileges to be denied to the container.
- `Add`: List of privileges to be allowed to the container.

The parameter is set at the container level in `spec.containers[].securityContext.capabilities.add` and `spec.containers[].securityContext.capabilities.drop` fields.

The final calculation of permitted privileges is made using the formula `(Default + Add) - Drop`, where `Default` is the standard set of container runtime (`containerd`/`CRIO`).

This means, operations denying privileges have priority. If you allow and deny the same privilege, the privilege will end up being denied.

{% alert level="warning" %}
The exception is processing at `drop: ALL`. In this case, all privileges are first removed and then only the ones listed in the `add` field are added.
{% endalert %}

### Available parameter values

The `Add` and `Drop` parameters are presented as an array of strings and are not validated at the Kubernetes level. It is acceptable to specify any values.
For a complete list of supported kernel privileges, refer to the [Linux documentation](https://man7.org/linux/man-pages/man7/capabilities.7.html).

Recommended practical approach:

1. Start with `drop: ["ALL"]`.
2. Add to `add` only the minimum required capabilities.

### Configuration examples

1. Same privileges in `add` and `drop`:

   ```yaml
   securityContext:
     capabilities:
       add: ["NET_ADMIN"]
       drop: ["NET_ADMIN"]
   ```

   As a result, the `NET_ADMIN` capability will be denied (the prohibition has a higher priority).

2. Adding one privilege without `drop`:

   ```yaml
   securityContext:
     capabilities:
       add: ["NET_BIND_SERVICE"]
   ```

   The final set of capabilities:

   - All capabilities from `Default` (standard set of container runtime).
   - And `NET_BIND_SERVICE` (if it was not in `Default`).

3. Removing part of the privileges:

   ```yaml
   securityContext:
     capabilities:
       drop: ["CHOWN", "FOWNER"]
   ```

   As a result, all capabilities from `Default` will be allowed (standard set of container runtime), except for `CHOWN` and `FOWNER`.

4. Hard restriction via `drop: ["ALL"]`:

   ```yaml
   securityContext:
     capabilities:
       drop: ["ALL"]
       add: ["NET_BIND_SERVICE"]
   ```

   As a result, only the `NET_BIND_SERVICE` capability is allowed.

## allowPrivilegeEscalation

The `allowPrivilegeEscalation` parameter determines whether a process inside a container can gain more privileges than its parent process.

It directly controls the Linux kernel system flag `no_new_privs` for the container being launched. If you set `allowPrivilegeEscalation: false`, Kubernetes enables this flag, disabling any privilege escalation mechanisms.

The main tool for elevating rights in Linux is files with the `SUID` (Set User ID) or `SGID` (Set Group ID) flags. When a regular user runs a SUID file, the process temporarily gains the rights of the owner of that file (often `root`).

For example, the `passwd` (for changing the password) or `sudo` utilities have the `SUID` flag. A normal user runs `sudo` and the Linux kernel temporarily escalates their privileges to superuser to perform a system task.

When using `allowPrivilegeEscalation: false`, the Linux kernel enables the `no_new_privs` mode. In this case:

- **SUID/SGID files are blocked**: When trying to run `sudo` or `passwd`, the kernel will ignore the `SUID` flag. The utility will be executed with the rights of the current container user and will generate the error `Permission denied`.
- **Activation of capabilities is prohibited**: The process will not be able to acquire new Linux Capabilities during its operation (for example, through executing files with preset file capabilities).

Even if the container is running as user `root` (UID `0`), Docker and Kubernetes by default truncate some system capabilities. However, if privilege escalation is available in the container, an attacker who gains access to the application can find a vulnerability in the Linux kernel or system utility within the container and use SUID calls to "escape the container" to the host node with full `root` privileges. Setting `allowPrivilegeEscalation: false` closes this attack vector.

{% alert level="warning" %}
If you make the container's file system read-only (`readOnlyRootFilesystem: true`), this indirectly prevents an attacker from creating his own SUID file, but does not protect against the use of utilities already existing in the image.
{% endalert %}

### Parameter location

The parameter is set in the `spec.containers[].securityContext.allowPrivilegeEscalation` field at the container level.

### Available parameter values

The parameter is of boolean type.
The following values ​​are available:

- `false`: Privilege escalation is prohibited (recommended for most applications).
- `true`: Privilege escalation is allowed.

### Configuration example

The following is an example of configuration a complete ban on elevating rights:

```yaml
spec:
  containers:
  - name: secure-app
    image: my-app:latest
    securityContext:
      allowPrivilegeEscalation: false 
```

## privileged

The `privileged` parameter determines whether the container runs with full kernel-level access to the host operating system on the node.

By setting the `privileged: true` parameter, Kubernetes completely disables all standard container isolation mechanisms. From a security point of view, this container becomes a normal process running as `root` directly on the host, albeit locked in its own network and cgroup space.

When setting the `privileged: true` parameter, the following changes occur:

- **Granting access to all capabilities**: The container receives all Linux Capabilities (about 40 pieces).
- The `add` and `drop` configurations in the `securityContext` for this container begin to be ignored.
- **Access to host devices**: All physical host devices (hard drives, NVMe drives, graphics cards, USB ports) appear inside the container in the `/dev` directory. The container can directly read and write to them.
- **Disabling AppArmor and SELinux**: Forced access control systems (`MAC`) on the host no longer impose restrictions on processes inside this container.
- Bypass `sysfs` and `procfs`: The container can freely change host kernel parameters through `/sys` and `/proc`.

The `privileged: true` flag is the biggest security hole if it is issued to an untrusted application. An attacker who gains access to such a container can be guaranteed to compromise the entire node with one command. For example, he can issue the `mount` command on the host's root disk and gain full control of the host's operating system files by "escaping the container".

{% alert level="warning" %}
This flag is strictly contraindicated for ordinary business applications. It is required exclusively for cluster system components: network plugins (CNI), storage drivers (CSI) or low-level monitoring agents.
{% endalert %}

### Parameter location

The parameter is set in the `spec.containers[].securityContext.privileged` field at the container level.

### Available parameter values

The parameter is of boolean type.
The following values ​​are available:

- `false`: Privileged mode disabled (default value, recommended for security).
- `true`: Privileged mode is enabled.

### Configuration example

The following is a configuration example for enabling maximum rights on the host for the system utility:

```yaml
spec:
  containers:
  - name: admin-tool
    image: ubuntu
    securityContext:
      privileged: true 
```

## procMount

The `procMount` parameter determines how subdirectories of the `/proc` system file system will be mounted inside the container.

It directly controls the system information isolation level of the Linux kernel. This option allows you to disable standard Kubernetes protection masks that hide critical and potentially dangerous host system paths from container processes.

The `/proc` file system in Linux is a window into the kernel. Through it you can not only read system metrics, but also change OS configuration parameters on the fly. By default, container runtimes (`containerd`, `CRI-O`) use protection masks (`MaskedPaths`) that hide or make critical paths read-only (for example, `/proc/sys`, `/proc/sysrq-trigger`, `/proc/scsi`).

For example, if a regular container needs to change network stack parameters via `sysctl`, the kernel protection mask will block writing to `/proc/sys/net`, displaying an error `Read-only file system`, to protect the host operating system from unauthorized changes.

When using `procMount`, Kubernetes issues instructions to the runtime to change the mount type. In this case:

- **Standard protection is enabled** (`Default`): All standard containerization masks are applied. Dangerous host system paths are hidden or write-blocked.
- **System isolation is removed** (`Unmasked`): All protective masks for the `/proc` file system are disabled. The container observes the host kernel structure "as is", without restrictions on reading and writing.

Setting the value to `Unmasked` allows direct access to the host kernel configuration. If an attacker compromises a container with masking disabled, he can exploit kernel vulnerabilities or directly change the host's system parameters (for example, cause an instant kernel panic or host reboot via `/proc/sysrq-trigger`). This parameter is the critical vector for "container escape".

{% alert level="warning" %}
The use of the `Unmasked` type is required extremely rarely. Its main use case is running specialized tools inside containers, such as image build tools (`Kaniko`, `Buildah`, etc.) or nested containers, which require full, unmodified access to `/proc` subsystems for process emulation.
{% endalert %}

### Parameter location

The parameter is set in the `spec.containers[].securityContext.procMount` field at the container level.

### Available parameter values

The parameter has a string type (`string`).
The following values ​​are available:

* `Default`: Standard masking of dangerous kernel paths (recommended by default).
* `Unmasked`: Disables protective masks and provides full access to `/proc`.

### Configuration example

The following is a configuration example of disabling `/proc` protections for a custom collector container:

```yaml
spec:
  containers:
  - name: image-builder
    image: kaniko-project/executor:latest
    securityContext:
      procMount: Unmasked
```

## sysctls

The `sysctls` parameter specifies a list of mutable Linux kernel parameters that can be safely or unsafely overridden for a particular pod's namespace.

It directly controls the behavior of the network stack, memory, and virtual file system at the container level. With this setting, you can optimize the performance of high-load applications (such as databases or web servers) without having to change global operating system settings on the entire host node.

In Linux, the `sysctl` utility allows you to change the kernel configuration while the system is running through the `/proc/sys/` interface. Kernel settings are divided into isolated (at the container namespace level) and global (affecting the entire physical node).

For example, by default, the maximum number of pending connections in the queue (`somaxconn`) is limited to a small system value. A heavily loaded NGINX traffic balancer may not have enough of this, causing it to start dropping packets. Using `sysctls`, a container can be individually allocated an increased queue size.

{% alert level="info" %}
In a DKP cluster, a number of `sysctls` parameters is configured automatically during the installation.
For a complete list of these parameters, refer to ["Sysctl parameters managed by the platform"](../../reference/sysctl.html).
{% endalert %}

When configuring `sysctls`, Kubernetes divides all parameters into two categories that require different levels of trust. In this case:

- **Safe parameters are allowed** (`Safe sysctls`): Parameters that are completely isolated inside the container and changing them cannot harm neighboring pods or the stability of the node. Kubernetes applies them immediately (for example, `net.ipv4.ip_local_port_range`).
- **Unsafe parameters are blocked** (`Unsafe sysctls`): Parameters that can affect the entire node (cause memory overload, disrupt overall routing). By default, Kubernetes will block a pod with these settings. For them to work, the cluster administrator must first allow them in the `kubelet` configuration via the `--allowed-unsafe-sysctls` flag.

The current list of safe `sysctl` settings is available in the [Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/).

Uncontrolled use of insecure `sysctls` can result in a denial of service (DoS) condition for the entire server. If an attacker gains access to a container with rights to change global network parameters, he can disrupt the connectivity of other pods on this node or clog up system memory, causing critical host processes to crash (`Out of Memory`).

{% alert level="warning" %}
There are only a few secure kernel parameters in Kubernetes (mainly `kernel.shm_rmid_forced` and part of the network parameters `net.ipv4.*` for the container's local network). If you need to use an Unsafe setting, the best practice is to move such applications to separate, isolated groups of nodes (dedicated node pools) using taints and tolerations to minimize the risk to the rest of the cluster.
{% endalert %}

### Parameter location

This parameter is set exclusively at the level of the entire pod in the `spec.securityContext.sysctls` field.

### Available parameter values

The parameter accepts a list of objects (`array`), where each element consists of a key-value pair (`name` and `value`) of type string (`string`).

### Configuration example

The following is a configuration example of local network stack parameters for a high-load pod:

```yaml
spec:
  securityContext:
    sysctls:
    - name: net.core.somaxconn
      value: "8192"
    - name: net.ipv4.ip_local_port_range
      value: "1024 65535"
  containers:
  - name: high-load-nginx
    image: nginx:latest
```

## hostNetwork

The `hostNetwork` parameter determines whether the pod will use the network namespace of the node it is running on, instead of a separate isolated network namespace.

By default, each pod receives its own network namespace (`netns`) with a virtual network interface, a separate IP address, and its own routing rules. Traffic is isolated from the host and neighboring pods by the network plugin (CNI). The `hostNetwork: true` setting disables this isolation.

When using `hostNetwork: true`, Kubernetes instructs the container runtime not to create a separate network namespace. In this case:

- **The host network stack is shared**: The container uses the node's network interfaces (`eth0`, `loopback`), its IP address, and routing tables directly.
- **Ports are opened on the host**: Any port that the application in the container listens on is automatically opened on the node's interfaces and is reachable from outside the cluster.
- **CNI is ignored**: The cluster network plugin does not assign a separate IP address to the pod and does not apply network policies (`NetworkPolicy`) to its traffic.

Enabling `hostNetwork: true` destroys the network isolation of the pod. The application gains direct access to all network interfaces of the node, can intercept foreign traffic (sniffing), interfere with cluster routing, or conflict on ports with system services of the host. Kubernetes network policies stop applying, since the pod's traffic is indistinguishable from the traffic of the node itself. An attacker who compromises such a pod can attack neighboring pods and nodes directly, bypassing micro-segmentation mechanisms.

{% alert level="warning" %}
`hostNetwork` is often used together with the `dnsPolicy: ClusterFirstWithHostNet` parameter. Without explicitly specifying this policy, a pod with `hostNetwork: true` will not be able to correctly resolve internal cluster service names through CoreDNS, because by default it will use the host's resolvers.
{% endalert %}

### Parameter location

The parameter is set in the `spec.hostNetwork` field exclusively at the level of the entire pod, since the network namespace is created for the pod as a whole.

### Available parameter values

The parameter is of boolean type.
The following values are available:

* `false`: The pod uses its own isolated network namespace (default value, recommended).
* `true`: The pod uses the network namespace of the node.

### Configuration example

The following is a configuration example of a pod using the node's network (for example, a system network agent):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: host-network-pod
spec:
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  containers:
    - name: network-agent
      image: my-agent:latest
```

## hostPID

The `hostPID` parameter determines whether the pod will use the PID namespace of the node instead of its own isolated one.

In Linux, process identifiers (PID) are unique only within a single namespace. By default, each pod receives its own PID namespace: processes inside the container see only themselves (PID `1` and its child processes), while processes of the host and neighboring pods are invisible to them. The `hostPID: true` setting disables this isolation.

When using `hostPID: true`, the container runtime does not create a separate PID namespace for the pod. In this case:

- **All node processes are visible**: Processes inside the container see the full list of processes running on the node (analogous to the `ps aux` command on the host).
- **Interaction with host processes is available**: Provided sufficient privileges, container processes can send signals (`kill`, `SIGTERM`) to node processes or inspect their memory via `/proc`.

The `hostPID: true` parameter opens a window into the node's operating system for container processes. An attacker who compromises such a pod can analyze system components running on the node (including `kubelet`, `containerd`, other pods), collect information about command lines and process arguments, and, with sufficient privileges, terminate foreign processes, causing a denial of service. In addition, access to `/proc` of foreign processes allows reading their environment variables, which often contain secrets and tokens.

{% alert level="warning" %}
The ability to send signals to host processes depends on the privileges of the user inside the container. Even with `hostPID: true`, a non-root container will not be able to directly manage `root` processes on the host, however, reading process metadata via `/proc` will still be available.
{% endalert %}

### Parameter location

The parameter is set in the `spec.hostPID` field exclusively at the level of the entire pod.

### Available parameter values

The parameter is of boolean type.
The following values are available:

* `false`: The pod uses its own isolated PID namespace (default value, recommended).
* `true`: The pod uses the PID namespace of the node.

### Configuration example

The following is a configuration example of a pod with access to node processes (for example, for system monitoring):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: host-pid-pod
spec:
  hostPID: true
  containers:
    - name: process-inspector
      image: my-tool:latest
```

## hostIPC

The `hostIPC` parameter determines whether the pod will use the IPC namespace of the node instead of its own isolated one.

In Linux, IPC mechanisms (message queues, shared memory, semaphores) are separated by namespaces. By default, each pod receives its own IPC namespace, isolated from the host and neighboring pods. This means that container processes cannot use IPC resources created by node processes or other pods. The `hostIPC: true` setting disables this isolation.

When using `hostIPC: true`, the container runtime uses the IPC namespace of the node. In this case:

- **Host IPC objects are shared**: Message queues, shared memory segments, and semaphores of the node become available to container processes.
- **Interprocess communication with the host is available**: The container can read and modify IPC objects created by system processes on the node.

The `hostIPC: true` parameter creates a data exchange channel between the container and node processes, bypassing standard network and file interfaces. An attacker who compromises such a pod can intercept or tamper with data exchanged by system services of the host via IPC, and can also inject malicious payloads into shared memory, exploiting vulnerabilities in processes that read these segments. In practice, the parameter is used extremely rarely — mostly for legacy applications that use System V IPC to synchronize with processes running directly on the node.

{% alert level="warning" %}
Most modern applications do not use System V IPC, preferring network sockets or standard file descriptors. Therefore, `hostIPC` can almost always be safely left at `false` without any loss of functionality.
{% endalert %}

### Parameter location

The parameter is set in the `spec.hostIPC` field exclusively at the level of the entire pod.

### Available parameter values

The parameter is of boolean type.
The following values are available:

* `false`: The pod uses its own isolated IPC namespace (default value, recommended).
* `true`: The pod uses the IPC namespace of the node.

### Configuration example

The following is a configuration example of a pod with access to the node's IPC resources:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: host-ipc-pod
spec:
  hostIPC: true
  containers:
    - name: ipc-client
      image: my-app:latest
```

## hostPath

The `hostPath` parameter determines the ability to mount arbitrary files and directories of the node's file system directly into the container.

Usually, containers work with data through isolated Kubernetes volumes (`emptyDir`, `PersistentVolume`, `configMap`), which do not give direct access to the node's disk. The `hostPath` volume type bypasses this abstraction and passes the specified path of the host file system inside the container.

When configuring `hostPath`, Kubernetes mounts the specified path from the node into the container. In this case:

- **The host file system is shared**: The container gets direct access to the files and directories of the node at the specified path (`path`).
- **Storage isolation is ignored**: The access is not limited by PersistentVolume or quotas. The container works with the real files of the node.
- The mount type is controlled by the `type` field: Available values are `""` (check disabled), `Directory`, `File`, `Socket`, `CharDevice`, and `BlockDevice` — they determine what type of object must exist at the path before mounting.

The `hostPath` parameter is one of the most dangerous volume types, as it opens direct access to the node's file system for the container. An attacker who gains access to such a container can read confidential host files (for example, `/etc/shadow`, private keys, `kubelet` tokens), modify system binaries, or replace the configuration of critical node services. Mounting root or system paths (`/`, `/var/lib/kubelet`, `/etc`, `/proc`, `/sys`) is essentially equivalent to granting full control over the node and is a direct path to "container escape".

{% alert level="warning" %}
When using `hostPath`, it is strongly recommended to mount the volume in read-only mode (`readOnly: true`) if the application does not require write access. The `type` field should also be specified explicitly, so that Kubernetes checks the existence and type of the object before mounting — this prevents the unintended creation of files on the host.
{% endalert %}

### Parameter location

The `hostPath` volume is described in the pod's volume array and then mounted into a specific container:

* `spec.volumes[].hostPath`: Description of a volume.
* `spec.containers[].volumeMounts`: Mount point inside the container.

### Available parameter values

The `hostPath` object contains the following fields:

* `path`: The absolute path in the node's file system that will be mounted (string, required field).
* `type`: The type of object at the specified path (string, optional). The following values are available:
  * `""`: Type check disabled (the default value).
  * `Directory`: The directory must exist.
  * `DirectoryOrCreate`: The directory will be created if it is missing.
  * `File`: The file must exist.
  * `FileOrCreate`: The file will be created if it is missing.
  * `Socket`: The UNIX socket must exist.
  * `CharDevice`: The character device must exist.
  * `BlockDevice`: The block device must exist.

### Configuration example

The following is a configuration example of mounting a node's configuration file in read-only mode:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hostpath-pod
spec:
  containers:
    - name: app
      image: my-app:latest
      volumeMounts:
        - mountPath: /host/etc/app.conf
          name: host-config
          readOnly: true
  volumes:
    - name: host-config
      hostPath:
        path: /etc/app/app.conf
        type: File
```

## automountServiceAccountToken

The `automountServiceAccountToken` parameter determines whether Kubernetes will automatically mount the ServiceAccount token into the container at the standard path.

In Kubernetes, each pod is associated with a ServiceAccount by default (if not specified explicitly, the `default` account is used). For authentication purposes, Kubernetes automatically generates a token for this service account and mounts it into each container of the pod at the path `/var/run/secrets/kubernetes.io/serviceaccount`. This token allows processes inside the container to access the Kubernetes API server on behalf of their ServiceAccount.

When using `automountServiceAccountToken: false`, Kubernetes does not mount the ServiceAccount token into the container. In this case:

- **The token file is absent**: No `token` file is created at the path `/var/run/secrets/kubernetes.io/serviceaccount`.
- **No API access on behalf of the ServiceAccount**: Container processes cannot automatically authenticate to the Kubernetes API server with the rights of their ServiceAccount.
- **Manual mounting remains possible**: If necessary, the token can be mounted explicitly through the `serviceAccountToken` volume type, or modern time-bound tokens (BoundServiceAccountTokenVolume), introduced in Kubernetes 1.21+, can be used.

An automatically mounted ServiceAccount token is a ready-made credential that an attacker can use for lateral movement across the cluster. If the application does not work with the Kubernetes API (for example, a regular web server or database), mounting the token creates an unnecessary risk: upon compromising the container, the attacker gains the ability to access the API server with the rights of the pod's ServiceAccount, including reading secrets, enumerating resources, and, with excessive rights (RBAC), attacking other cluster components. Disabling automatic mounting implements the principle of least privilege and reduces the attack surface.

{% alert level="warning" %}
The `automountServiceAccountToken` parameter can be set both at the ServiceAccount level and at the pod level (`spec.automountServiceAccountToken`). The value specified in the pod takes precedence over the value in the ServiceAccount. It is also important to make sure that the application does not actually use the Kubernetes API before disabling token mounting.
{% endalert %}

### Parameter location

The parameter is set:

* `spec.automountServiceAccountToken`: At the level of the entire pod.
* `serviceAccount.automountServiceAccountToken`: At the ServiceAccount resource level.

### Available parameter values

The parameter is of boolean type.
The following values are available:

* `true`: The ServiceAccount token is mounted automatically (default value unless otherwise specified).
* `false`: Automatic token mounting is disabled (recommended for pods that do not interact with the Kubernetes API).

### Configuration example

The following is a configuration example of a pod with automatic service account token mounting disabled:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: no-sa-token-pod
spec:
  automountServiceAccountToken: false
  containers:
    - name: app
      image: my-app:latest
```
