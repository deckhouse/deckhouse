sequenceDiagram
    participant Deckhouse-controller
    participant main@{ "type" : "queue" }
    participant parallel_queue_0@{ "type" : "queue" }
    participant parallel_queue_1@{ "type" : "queue" }
    participant parallel_queue_2..19@{ "type" : "queue" }
    
    loop for every global hook
        Deckhouse-controller->>+main: Run GlobalHookRun
        main-->>-Deckhouse-controller: GlobalHookRun executed
    end
    loop for every module
        Deckhouse-controller->>+main: Run ModuleEnsureCRDs
        main-->>-Deckhouse-controller: ModuleEnsureCRDs executed
    end
    
    par Module A installation
        Deckhouse-controller->>+parallel_queue_0: "Run ModuleRun A"                

        break Module installation fails
            parallel_queue_0-->>Deckhouse-controller: Reschedule installation at the end of queue
        end

        parallel_queue_0-->>-Deckhouse-controller: "Module A installed"

    and Module B installation
        Deckhouse-controller->>+parallel_queue_1: "Run ModuleRun B"
        parallel_queue_1-->>-Deckhouse-controller: "Module B installed"

    and Other modules installation
        Deckhouse-controller->>+parallel_queue_2..19: "Run ModuleRun C, ..."
        
        parallel_queue_2..19-->>-Deckhouse-controller: "Module C, ... installed"
    end
