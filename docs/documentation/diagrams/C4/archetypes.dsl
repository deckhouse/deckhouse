archetypes {
    subsystem = container {
        tag "Subsystem"
        description "Подсистема"
    }

    module = container {
        tag "Module"
        description "Модуль"
    }

    daemon = container {
        tag "Daemon"
        description "Служба"
    }

    database = container {
        tag "Database"
        description "База данных"
    }

    staticPodGroup = container {
        tag "StaticPodGroup"
        description "Cтатические поды"
    }

    application = container {
        tag "Application"
        description "Приложение"
    }

    pod = container {
        tag "Pod"
    }

    files = container {
        tag "Files"
        description "Файлы на узлах"
    }

    external = element {
        tag "External"
    }

    example-system = element {
        tag "dkp"
    }

    example-subsystem = element {
        tag "Subsystem"
    }

    example-module = element {
        tag "Module"
    }

    example-container = element {
        tag "Container"
    }

    example-database = element {
        tag "Database"
    }

    example-daemon = element {
        tag "Daemon"
    }

    example-files = element {
        tag "Files"
    }

}