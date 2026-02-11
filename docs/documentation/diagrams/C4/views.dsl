systemContext dkp "L1_context" {
    include *
    exclude network-load-balancer
}

container dkp "L2_subsystems" {
    include users
    include alert-receivers
    include authn-providers
    include cert-issuers
    include iaas
    include logging-external
    include ntp-servers
    include registry-external
    include network-load-balancer
    include network-infra
    include storage-infra
    include dkp.cluster-and-infrastructure
    include dkp.deckhouse-subsystem
    include dkp.delivery
    include dkp.iam
    include dkp.kubernetes-and-scheduling
    include dkp.managed-services
    include dkp.network
    include dkp.observability-subsystem
    include dkp.security
    include dkp.storage
    include dkp.virtualization-subsystem
}

container dkp "L2_modules" {           
    include *
    include users
    exclude dkp.group-user-app
    exclude dkp.group-user-app-with-dex-client
    exclude dkp.group-log-sources
    exclude dkp.group-log-destinations
    exclude dkp.kubelet
    exclude dkp.containerd
    exclude dkp.bashible
    exclude dkp.cloud-provider
    // Исключаем подсистемы (как элементы)
    exclude dkp.cluster-and-infrastructure
    exclude dkp.deckhouse-subsystem
    exclude dkp.delivery
    exclude dkp.iam
    exclude dkp.kubernetes-and-scheduling
    exclude dkp.managed-services
    exclude dkp.network
    exclude dkp.observability-subsystem
    exclude dkp.security
    exclude dkp.storage
    exclude dkp.virtualization-subsystem
    // Исключаем модули и статические поды (как группы)
    exclude dkp.static-pod-group-cluster-control-plane
    exclude dkp.module-console
    exclude dkp.module-terraform-manager
    exclude dkp.module-control-plane-manager
    exclude dkp.module-ingress-nginx
    exclude dkp.module-log-shipper
    exclude dkp.module-loki
    exclude dkp.module-monitoring-kubernetes-control-plane
    exclude dkp.module-node-manager
    exclude dkp.module-prometheus
    exclude dkp.module-user-authn
}

// Подсистема Cluster & Infrastructure
container dkp "L2_module_node-manager-cloud-ephemeral" {
    include iaas
    include dkp.pod-bashible-apiserver
    include dkp.pod-capi-controller-manager
    include dkp.pod-cluster-autoscaler
    include dkp.pod-early-oom
    include dkp.pod-fencing-agent
    include dkp.pod-fencing-controller
    include dkp.pod-standby-holder
    include dkp.watchdog-file
    include dkp.kube-apiserver
    include dkp.prometheus-main
    include dkp.bashible
    include dkp.proc-files
    include dkp.module-cloud-provider
}

container dkp "L2_module_node-manager-cloud-permanent" {
    include user
    include iaas
    include dhctl
    include dkp.csi-driver
    include dkp.cloud-controller-manager
    include dkp.pod-bashible-apiserver
    include dkp.pod-early-oom
    include dkp.pod-fencing-agent
    include dkp.pod-fencing-controller
    include dkp.watchdog-file
    include dkp.kube-apiserver
    include dkp.prometheus-main
    include dkp.terraform-manager
    include dkp.bashible
    include dkp.proc-files
}

container dkp "L2_module_node-manager-cloud-static" {
    include user-static-node
    include iaas
    include dkp.csi-driver
    include dkp.cloud-controller-manager
    include dkp.pod-bashible-apiserver
    include dkp.pod-capi-controller-manager
    include dkp.pod-caps-controller-manager
    include dkp.pod-early-oom
    include dkp.pod-fencing-agent
    include dkp.pod-fencing-controller
    include dkp.watchdog-file
    include dkp.kube-apiserver
    include dkp.prometheus-main
    include dkp.bashible
    include dkp.proc-files
}

container dkp "L2_module_node-manager-static" {
    include user-static-node
    include iaas
    include dkp.pod-bashible-apiserver
    include dkp.pod-capi-controller-manager
    include dkp.pod-caps-controller-manager
    include dkp.pod-early-oom
    include dkp.pod-fencing-agent
    include dkp.pod-fencing-controller
    include dkp.watchdog-file
    include dkp.kube-apiserver
    include dkp.prometheus-main
    include dkp.bashible
    include dkp.proc-files
}

container dkp "L2_module_control_plane_manager" {
    include dkp.module-control-plane-manager
    include dkp.static-pod-group-cluster-control-plane
    include dkp.prometheus-main
    include dkp.module-monitoring-kubernetes-control-plane
    include dkp.etcd-backup-files
    include dkp.kubelet
}

container dkp "L2_module_terraform-manager" {
    include iaas
    include dkp.module-terraform-manager
    include dkp.kube-apiserver
    include dkp.prometheus-main
}

container dkp "L2_module_ingress-nginx" {
    include dkp.module-ingress-nginx
    include users
    include authn-providers
    include network-load-balancer
    include dkp.console-dex-authenticator
    include dkp.dex
    // include dkp.etcd
    include dkp.frontend
    include dkp.kube-apiserver
    // include dkp.kubeconfig-generator // Deprecated maksim.nabokikh@flant.com
    include dkp.prometheus-main
    include dkp.user-app
    include dkp.user-app-dex-authenticator
}

container dkp "L2_module_user-authn" {
    include dkp.module-user-authn
    include users
    include authn-providers
    include network-load-balancer
    include dkp.ing-controller
    include dkp.group-user-app
    include dkp.kube-apiserver
    exclude dkp.pod-console-dex-authenticator
    exclude dkp.pod-dashboard-dex-authenticator
    exclude dkp.pod-grafana-dex-authenticator
    exclude dkp.pod-deckhouse-tools-dex-authenticator
    exclude dkp.pod-documentation-dex-authenticator
    exclude dkp.pod-status-dex-authenticator 
    exclude dkp.pod-upmeter-dex-authenticator
    include dkp.prometheus-main
}

container dkp "L2_module_user-authn-with-dex-client" {
    include dkp.module-user-authn
    include users
    include authn-providers
    include network-load-balancer
    include dkp.ing-controller
    // include dkp.prometheus-main 
    // include dkp.group-user-app
    include dkp.group-user-app-with-dex-client
    exclude dkp.pod-console-dex-authenticator
    exclude dkp.pod-dashboard-dex-authenticator
    exclude dkp.pod-grafana-dex-authenticator
    exclude dkp.pod-deckhouse-tools-dex-authenticator
    exclude dkp.pod-documentation-dex-authenticator
    exclude dkp.pod-status-dex-authenticator 
    exclude dkp.pod-upmeter-dex-authenticator
    exclude dkp.pod-user-app-dex-authenticator
    exclude dkp.dex-kube-rbac-proxy
}

container dkp "L2_module_log-shipper" {
    include logging-external
    include dkp.module-log-shipper
    include dkp.prometheus-main
    include dkp.kube-apiserver
    include dkp.group-log-destinations
    include dkp.group-log-sources
}

container dkp "L2_module_loki" {
    include dkp.vector
    include dkp.module-loki
    include dkp.console    
    include dkp.grafana-v10
    include dkp.prometheus-main
}

container dkp "Legend" {
    include example-system
    include example-subsystem
    include example-module
    include example-container
    include example-static-pod
    include example-database
    include example-daemon
    include example-external
    include example-files
}


