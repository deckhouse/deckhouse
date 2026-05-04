## Deploying a test application

To showcase Deckhouse Virtualization Platform, deploy the demo application. It is a set of virtual machines and containers in a classic three-tier layout (Frontend → Backend → Database).

### What the application demonstrates

- **Declarative deployment and management**: platform resources, users, and workloads via YAML manifests
- **Multitenancy and RBAC**: isolation between projects and access control for different users
- **VMs and containers together**: VMs and container workloads in the same cluster
- **Network microsegmentation**: network policies to control traffic between components
- **Load balancing**: Ingress-based request distribution across instances
- **DEX integration**: example authentication through a DEX provider

## Application architecture

![Platform architecture](/images/virtualization-platform/dvp-gs-architecture-test-app.png)

The application spans two projects:

**Project: `demo-db`**
- **Database**: VM `db` with PostgreSQL

**Project: `demo-app`**
- **Frontend**:
  - VM `frontend` (Bootstrap app)
  - Pod `frontend` (Bootstrap app)
- **Backend**:
  - VM `backend-a` (Flask + Gunicorn)
  - VM `backend-b` (Flask + Gunicorn)

Component traffic is constrained by network policies. Access requires DEX authentication.

## Requirements

Before installing the demo application, ensure the following.

### Installed packages

On your workstation you need:

- `task` — task runner
- `yq` — YAML processor

## Installing the application

1. **Clone** the manifests repository:

   ```bash
   git clone https://github.com/fl64/dvp-demo-app.git
   cd dvp-demo-app
   ```

1. **Create a `.env` file** with infrastructure settings:

   ```bash
   cat > .env <<EOF
   STORAGE_CLASS=nfs-storage-class
   PASSWORD=password
   FQDN=demo.example.com
   EOF
   ```

   Replace the values to match your environment:
   - `STORAGE_CLASS` — NFS StorageClass name, the same as in `config.yml` from the installation step (cluster parameters; default `nfs-storage-class`)
   - `PASSWORD` — password for application users
   - `FQDN` — hostname for accessing the application

1. **Generate an SSH key** for VM access:

   ```bash
   task ssh-gen
   ```

1. **Deploy** the application:

   ```bash
   task deploy
   ```

1. Wait until components are ready. Check VMs:

   ```bash
   sudo -i d8 k get vm -A
   ```

   Check pods:

   ```bash
   sudo -i d8 k get po -A | grep demo
   ```

### Removing the application

```bash
task undeploy
```

## Connecting to virtual machines

### SSH

```bash
sudo -i d8 v ssh -n demo-app cloud@<vmname> -i ./tmp/demo --local-ssh
```

Replace `<vmname>` with the VM name (for example `frontend`, `backend-a`, `backend-b`, `db`).

### Console

```bash
sudo -i d8 v console -n demo-app <vmname>
```

Replace `<vmname>` with the VM name.

## Walkthrough

After a successful deployment:

1. **Open the application UI** in the browser at the address from the Ingress resource.
1. **Sign in via DEX** using the credentials created during setup; verify authentication works.
1. **Check load balancing**: refresh several times and confirm traffic hits both the `frontend` VM and the `frontend` pod.
1. **Exercise the three-tier flow**: add a field in the UI, remove it, and confirm persistence in the database.
1. **Explore the Deckhouse console**: review projects `demo-db` and `demo-app`, VMs, application pods, load balancer configuration, and network policies.

You have deployed the demo application and explored the main capabilities of Deckhouse Virtualization Platform.
