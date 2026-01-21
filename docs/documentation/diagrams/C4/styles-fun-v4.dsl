theme default

styles {
    relationship "Relationship" {
        thickness 1
        color #000000
        style solid
        routing direct
    }

    element "Subsystem" {
        shape RoundedBox
        background #E8EFFF
        color #4B6CA0
        stroke #88A6FF
        strokeWidth 1 
    }

    element "Module" {
        // background #457cd4
        // color #FFFFFF
        shape RoundedBox
        background #D8E4FF
        color #325EBD
        stroke #D8E4FF
        strokeWidth 1 
    }

    element "Container" {
        // background #94B3E6
        // background #c6d7f1
        // color #000000
        shape RoundedBox
        background #D8E4FF
        color #000000
        stroke #485D8E
        strokeWidth 1 
    }
    
    element "Database" {
        // background #94B3E6
        // background #c6d7f1
        shape cylinder
        background #D8E4FF
        color #000000
        stroke #485D8E
        strokeWidth 1 
    }

    element "StaticPodGroup" {
        shape RoundedBox
        background #FFFFFF
        color #000000
        stroke #FFFFFF
        strokeWidth 1 
    }

    element "Daemon" {
        // background #ea3b52
        // background #D3CCF8
        shape RoundedBox
        background #EFECFF
        color #000000
        stroke #735FE6
        strokeWidth 1 
    }

    element "Files" {
        shape cylinder
        // background #D3CCF8
        shape Cylinder
        background #EFECFF
        color #3E26CE
        stroke #735FE6
        strokeWidth 1   
    }

    element "External" {
        // background #cacaca
        // color #000000
        shape RoundedBox
        background #EDEDED
        color #000000
        stroke #696A6D
        strokeWidth 1 
    }

    element "Person" {
        // background #485d8e
        shape RoundedBox
        background #8F7FEB
        color #FFFFFF
        stroke #8F7FEB
        strokeWidth 1    
    }

    element "dkp" {
        shape RoundedBox
        background #FFFFFF
        color #004DF2
        stroke #004DF2
        strokeWidth 1  
    }

    element "Group:Пользовательское приложение" {
        background #c6d7f1
    }

    element "Group:Пользовательское приложение (с клиентом Dex)" {
        background #c6d7f1
    }

    element "Group:Источники логов в кластере/Приложение в кластере" {
        background #c6d7f1
    }

    element "Group:Подсистема Cluster & Infrastructure" {
        // background #c6d7f1
        background #e8eff9
    }   

    element "Group:Подсистема Deckhouse" {
        background #e8eff9
    }

    element "Group:Подсистема Delivery" {
        background #e8eff9        
    }

    element "Group:Подсистема IAM" {
        background #e8eff9
    }

    element "Group:Подсистема Kubernetes & Scheduling" {
        background #e8eff9               
    } 

    element "Group:Подсистема Managed Services" {
        background #e8eff9
    }

    element "Group:Подсистема Network" {
        background #e8eff9
    }

    element "Group:Подсистема Observability" {
        background #e8eff9
    }

    element "Group:Подсистема Security" {
        background #e8eff9
    }

    element "Group:Подсистема Storage" {
        background #e8eff9
    }

    element "Group:Подсистема Virtualization" {
        background #e8eff9
    }

    element "Group:Подсистема Cluster & Infrastructure/Модуль node-manager" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Cluster & Infrastructure/Модуль terraform-manager" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Cluster & Infrastructure/Модуль cloud-provider-*" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Kubernetes & Scheduling/Сontrol plane Kubernetes-кластера" {
        // background #b2d4ec
        background #c6d7f1
    }

    element "Group:Подсистема Kubernetes & Scheduling/Модуль control-plane-manager" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема IAM/Модуль user-authn" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }
    
    element "Group:Подсистема Network/Модуль ingress-nginx" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Observability/Модуль log-shipper" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Observability/Модуль loki" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Observability/Модуль monitoring-kubernetes-control-plane" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Observability/Модуль prometheus" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }

    element "Group:Подсистема Deckhouse/Модуль console" {
        // background #263360
        // color #FFFFFF
        background #c6d7f1
    }
}
