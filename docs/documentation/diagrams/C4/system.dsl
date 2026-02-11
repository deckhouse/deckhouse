dkp = softwareSystem "Deckhouse Kubernetes Platform" "Варианты инсталляции для разных типов инфраструктуры " "dkp" {
    cluster-and-infrastructure  = subsystem "Cluster & Infrastructure"
    deckhouse-subsystem         = subsystem "Deckhouse"
    delivery                    = subsystem "Delivery"
    iam                         = subsystem "IAM"
    kubernetes-and-scheduling   = subsystem "Kubernetes & Scheduling"
    managed-services            = subsystem "Managed Services"
    network                     = subsystem "Network"
    observability-subsystem     = subsystem "Observability"    
    security                    = subsystem "Security"
    storage                     = subsystem "Storage"
    virtualization-subsystem    = subsystem "Virtualization"

    group-user-app = group "Пользовательское приложение" {
        pod-user-app = group "user-app [pod]" {
            user-app = container "user-app"
        }
    }

    group-user-app-with-dex-client = group "Пользовательское приложение (с клиентом Dex)" {
        pod-user-app-with-dex-client = group "user-app-with-dex-client [pod]" {
            user-app-with-dex-client = container "user-app-with-dex-client"
        }
    }
      
    subsystem-cluster-and-infrastructure = group "Подсистема Cluster & Infrastructure" {
        chrony                      = module "chrony"
        node-manager                = module "node-manager"
        registry-packages-proxy     = module "registry-packages-proxy"
        terraform-manager           = module "terraform-manager"
        cloud-providers             = module "cloud-providers\n[11 модулей]"
        cloud-provider              = module "cloud-provider-*"
        bashible                    = daemon "bashible"

        module-cloud-provider = group "Модуль cloud-provider-*" {
            infrastructure-provider  = application "infrastructure-provider"
            cloud-controller-manager = application "cloud-controller-manager"
            csi-driver               = application "csi-driver"
        }

        module-node-manager = group "Модуль node-manager" {
            pod-bashible-apiserver = group "bashible-apiserver [pod]" {
                bashible-apiserver = container "bashible-apiserver"
            }

            pod-capi-controller-manager = group "capi-controller-manager [pod]" {
                capi-controller-manager = container "capi-controller-manager"
                capi-kube-rbac-proxy = container "kube-rbac-proxy (capi)"
            }

            pod-caps-controller-manager = group "caps-controller-manager [pod]" {
                caps-controller-manager = container caps-controller-manager
            }

            pod-cluster-autoscaler = group "cluster-autoscaler [pod]" {
                cluster-autoscaler = container "cluster-autoscaler"
                ca-kube-rbac-proxy = container "kube-rbac-proxy (ca)"
            }

            pod-early-oom = group "early-oom [pod]" {
                psi-monitor = container "psi-monitor"
                oom-kube-rbac-proxy = container "kube-rbac-proxy (oom)"
            }

            pod-fencing-agent = group "fencing-agent (опционально) [pod]" {
                fencing-agent = container "fencing-agent"
            }

            pod-fencing-controller = group "fencing-controller (опционально) [pod]" {
                fencing-controller = container "fencing-controller"
            }

            // Deprected
            // pod-machine-controller-manager = group "machine-controller-manager [pod]" {
            //      = container ""
            //      = container ""
            // }

            pod-standby-holder = group "standby-holder (опционально) [pod]" {
                reserve-resources = container "reserve-resources"
            }
        }

        module-terraform-manager = group "Модуль terraform-manager" { 
            pod-terraform-auto-converger = group "terraform-auto-converger [pod]" {
                terraform-auto-converger-to-tofu-migrator = container "to-tofu-migrator (initContainer)"
                terraform-auto-converger = container "converger"
                terraform-auto-converger-kube-rbac-proxy = container "kube-rbac-proxy (terraform-auto-converger)"
            }

            pod-terraform-state-exporter = group "terraform-state-exporter [pod]" {
                terraform-state-exporter = container "exporter"
                terraform-state-exporter-kube-rbac-proxy = container "kube-rbac-proxy (terraform-state-exporter)"
            }
        }       
     }

    proc-files      = files "/proc"
    watchdog-file   = files "/dev/watchdog"

    subsystem-deckhouse = group "Подсистема Deckhouse" {
        console             = module "console"
        dashboard           = module "dashboard"
        deckhouse           = module "deckhouse"
        deckhouse-tools     = module "deckhouse-tools"
        documentation       = module "documentation"
        registry            = module "registry"

        module-console = group "Модуль console" {
            pod-frontend = group "frontend [pod]" {
                frontend = container "frontend"
            }
        }
    }

    subsystem-delivery = group "Подсистема Delivery" {
        pod-reloader = module "pod-reloader"
    }

    subsystem-iam = group "Подсистема IAM" {
        multitenancy-manager    = module "multitenancy-manager"
        namespace-configurator  = module "namespace-configurator"
        user-authn              = module "user-authn"
        user-authz              = module "user-authz"

        module-user-authn = group "Модуль user-authn" {
            pod-dex = group "dex [pod]" {
                dex                 = container "dex"
                dex-kube-rbac-proxy = container "kube-rbac-proxy (dex)"
            }

            // To be deprected soon
            // pod-kubeconfig-generator = group "kubeconfig-generator [pod]" {
            //     kubeconfig-generator = container "kubeconfig-generator"
            // } 

            pod-console-dex-authenticator = group "console-dex-authenticator [pod]" {
                console-dex-self-signed-generator   = container "self-signed-generator (console-dex)"
                console-dex-authenticator           = container "dex-authenticator (console-dex)"
                console-dex-redis                   = database "redis (console-dex)"
            }

            pod-dashboard-dex-authenticator = group "dashboard-dex-authenticator [pod]" {
                dashboard-dex-self-signed-generator   = container "self-signed-generator (dashboard-dex)"
                dashboard-dex-authenticator           = container "dex-authenticator (dashboard-dex)"
                dashboard-dex-redis                   = database "redis (dashboard-dex)"
            }

            pod-grafana-dex-authenticator = group "grafana-dex-authenticator [pod]" {
                grafana-dex-self-signed-generator   = container "self-signed-generator (grafana-dex)"
                grafana-dex-authenticator           = container "dex-authenticator (grafana-dex)"
                grafana-dex-redis                   = database "redis (grafana-dex)"
            }

            pod-deckhouse-tools-dex-authenticator = group "deckhouse-tools-dex-authenticator [pod]" {
                deckhouse-tools-dex-self-signed-generator   = container "self-signed-generator (deckhouse-tools-dex)"
                deckhouse-tools-dex-authenticator           = container "dex-authenticator (deckhouse-tools-dex)"
                deckhouse-tools-dex-redis                   = database "redis (deckhouse-tools-dex)"
            }

            pod-documentation-dex-authenticator = group "documentation-dex-authenticator [pod]" {
                documentation-dex-self-signed-generator   = container "self-signed-generator (documentation-dex)"
                documentation-dex-authenticator           = container "dex-authenticator (documentation-dex)"
                documentation-dex-redis                   = database "redis (documentation-dex)"
            }  

            pod-status-dex-authenticator = group "status-dex-authenticator [pod]" {
                status-dex-self-signed-generator   = container "self-signed-generator (status-dex)"
                status-dex-authenticator           = container "dex-authenticator (status-dex)"
                status-dex-redis                   = database "redis (status-dex)"
            }  

            pod-upmeter-dex-authenticator = group "upmeter-dex-authenticator [pod]" {
                upmeter-dex-self-signed-generator   = container "self-signed-generator (upmeter-dex)"
                upmeter-dex-authenticator           = container "dex-authenticator (upmeter-dex)"
                upmeter-dex-redis                   = database "redis (upmeter-dex)"
            }  

            pod-user-app-dex-authenticator = group "user-app-dex-authenticator [pod]" {
                user-app-dex-self-signed-generator  = container "self-signed-generator (user-app-dex)"
                user-app-dex-authenticator          = container "dex-authenticator (user-app-dex)"
                user-app-dex-redis                  = database "redis (user-app-dex)"
            } 
        }        
    }

    subsystem-kubernetes-and-scheduling = group "Подсистема Kubernetes & Scheduling" {
        cluster-control-plane   = staticPodGroup "Сontrol plane Kubernetes-кластера" "Cтатические поды"                
        containerd              = daemon "containerd"
        control-plane-manager   = module "control-plane-manager"
        descheduler             = module "descheduler"
        kubelet                 = daemon "kubelet"
        vertical-pod-autoscaler = module "vertical-pod-autoscaler"

        module-control-plane-manager = group "Модуль control-plane-manager" {
            pod-d8-control-plane-manager = group "d8-control-plane-manager [pod]" {
                cpm-control-plane-manager               = container "control-plane-manager (d8)"
                image-holder-etcd                       = container "image-holder-etcd"
                image-holder-kube-apiserver             = container "image-holder-kube-apiserver"
                image-holder-kube-apiserver-healthcheck = container "image-holder-kube-apiserver-healthcheck"
                image-holder-kube-controller-manager    = container "image-holder-kube-controller-manager"
                image-holder-kube-scheduler             = container "image-holder-kube-scheduler"
            }

            pod-kubernetes-api-proxy = group "kubernetes-api-proxy [pod]" {
                kubernetes-api-proxy = container "kubernetes-api-proxy"
                kubernetes-api-proxy-reloader = container "kubernetes-api-proxy-reloader"
            }

            cronjob-d8-etcd-backup = group "d8-etcd-backup [cronjob]" {
                etcd-backup = container "backup (etcd)"
            }
        }

        static-pod-group-cluster-control-plane = group "Сontrol plane Kubernetes-кластера" {
            pod-kube-apiserver = group "kube-apiserver [pod]" {
                kube-apiserver = container "kube-apiserver"
                kube-apiserver-healthcheck = container "healthcheck (kube-apiserver)"
            }

            pod-kube-controller-manager = group "kube-controller-manager [pod]" {
                kube-controller-manager = container "kube-controller-manager"
            }

            pod-kube-scheduler = group "kube-scheduler [pod]" {
                kube-scheduler = container "kube-scheduler"
            }

            pod-etcd = group "etcd [pod]" {
                etcd = database "etcd"
            }
        }
    }

    etcd-backup-files = files "/var/lib/etcd"


    subsystem-managed-services = group "Подсистема Managed Services" {
        managed-postgres = module "managed-postgres"
    }

    subsystem-network = group "Подсистема Network" {
        ingress-nginx           = module "ingress-nginx"
        kube-dns                = module "kube-dns"
        node-local-dns          = module "node-local-dns"                
        kube-proxy              = module "kube-proxy"
        monitoring-ping         = module "monitoring-ping"
        metallb                 = module "metallb"
        cni-cilium              = module "cni-cilium"

        module-ingress-nginx = group "Модуль ingress-nginx" {
            pod-controller-nginx = group "controller-nginx [pod]" {
                ing-controller          = container "controller (nginx)"
                ing-protobuf-exporter   = container "protobuf-exporter (nginx)"
                ing-kube-rbac-proxy     = container "kube-rbac-proxy (nginx)"
                ing-istio-proxy         = container "istio-proxy (nginx)\n[опционально]"
            }

            pod-kruise-controller-manager = group "kruise-controller-manager [pod]" {
                kcm-kruise                  = container "kruise"
                kcm-kube-rbac-proxy         = container "kube-rbac-proxy (kruise)"
                kcm-kruise-state-metrics    = container "kruise-state-metrics"
            } 

            pod-validator-nginx = group "validator-nginx [pod]" {
                ing-validator = container "validator (nginx)"
            } 

            pod-failover-cleaner = group "failover-cleaner [pod]" {
                ing-failover-cleaner = container "NODE_NAME (nginx)"
            } 
        }
    }

    subsystem-observability = group "Подсистема Observability" {
        extended-monitoring                 = module "extended-monitoring"
        log-shipper                         = module "log-shipper"
        monitoring-custom                   = module "monitoring-custom"
        monitoring-deckhouse                = module "monitoring-deckhouse"
        monitoring-kubernetes               = module "monitoring-kubernetes"
        monitoring-kubernetes-control-plane = module "monitoring-kubernetes-control-plane"
        observability                       = module "observability"                  
        operator-prometheus                 = module "operator-prometheus"
        prometheus                          = module "prometheus"
        prometheus-metrics-adapter          = module "prometheus-metrics-adapter"                
        prompp                              = module "prompp"
        upmeter                             = module "upmeter"

        module-log-shipper = group "Модуль log-shipper" {
            pod-log-shipper-agent = group "log-shipper-agent [pod]" {
                vector                  = container "vector"
                vector-reloader         = container "vector-reloader"
                vector-kube-rbac-proxy  = container "kube-rbac-proxy (vector)"
            }
        }

        module-loki = group "Модуль loki" {
            pod-loki = group "loki [pod]" {
                loki                    = database "loki"
                loki-kube-rbac-proxy    = container "kube-rbac-proxy (loki)"
            }
        }

       module-monitoring-kubernetes-control-plane = group "Модуль monitoring-kubernetes-control-plane" {
            pod-control-plane-proxy = group "control-plane-proxy [pod]" {
                cpl-kube-rbac-proxy    = container "kube-rbac-proxy (control-plane)"
            }
        }

        module-prometheus = group "Модуль prometheus" {
            pod-grafana-v10 = group "grafana-v10 [pod]" {
                grafana-v10 = container "grafana (v10)"
            }

            pod-prometheus-main = group "prometheus-main [pod]" {
                prometheus-main = container "prometheus (main)"
            }
        }
    }

    group-log-sources = group "Источники логов в кластере" {
        log-files = files "Файлы"

        group-log-user-app = group "Приложение в кластере" {       
            pod-log-user-app = group "user-app (log source) [pod]" {
                log-user-app = container "user-app (log source)"
            }
        }
    }

    group-log-destinations = group "Приемники логов в кластере" {     
        elasticsearch = application "Elasticsearch"
        logstash = application "Logstash"
        kafka = application "Kafka"
        splunk = application "Splunk"
        loki-custom = application "Loki (отдельная инсталляция)"
    }

    subsystem-security = group "Подсистема Security" {
        admission-policy-engine     = module "admission-policy-engine"
        cert-manager                = module "cert-manager"
        secrets-store-integration   = module "secrets-store-integration"
        secret-copier               = module "secret-copier"
        operator-trivy              = module "operator-tryvy"
        runtime-audit-engine        = module "runtime-audit-engine"
    }

    subsystem-storage = group "Подсистема Storage" {
        local-path-provisioner  = module "local-path-provisioner"
        snapshot-controller     = module "snapshot-controller"
        csi-modules             = module "CSI\n[8 модулей]"
    }

    subsystem-virtualization = group "Подсистема Virtualization" {
        virtualization          = module "virtualization"
    }
}