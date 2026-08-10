sequenceDiagram
    participant Deckhouse-controller
    participant main@{ "type" : "queue" }
    participant parallel_queue_0@{ "type" : "queue" }
    participant parallel_queue_1@{ "type" : "queue" }
    participant parallel_queue_2..19@{ "type" : "queue" }
    
    loop для каждого глобального хука
        Deckhouse-controller->>+main: Запуск GlobalHookRun
        main-->>-Deckhouse-controller: GlobalHookRun выполнено
    end
    loop для каждого модуля
        Deckhouse-controller->>+main: Запуск ModuleEnsureCRDs
        main-->>-Deckhouse-controller: ModuleEnsureCRDs выполнено
    end
        
    par Установка модуля A
        Deckhouse-controller->>+parallel_queue_0: Запуск ModuleRun A        
        break Ошибка при установке модуля
            parallel_queue_0-->>Deckhouse-controller: Повторный запуск установки в конце очереди
        end
        parallel_queue_0-->>-Deckhouse-controller: Модуль A установлен

    and Установка модуля B
        Deckhouse-controller->>+parallel_queue_1: Запуск ModuleRun B                                 
        parallel_queue_1-->>-Deckhouse-controller: Модуль B установлен

    and Установка других модулей
        Deckhouse-controller->>+parallel_queue_2..19: Запуск ModuleRun C, ...
        parallel_queue_2..19-->>-Deckhouse-controller: Модули C, ... установлены
    end
