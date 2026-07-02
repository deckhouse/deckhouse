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

    Deckhouse-controller->>+main: Run ParallelModuleRun / <br/>Block main queue
    par Module A installation
        main->>+parallel_queue_0: Run ModuleRun A        
        break Module installation fails
            parallel_queue_0-->>main: Reschedule installation at the end of queue
        end
        parallel_queue_0-->>-main: Module A installed
    and Module B installation
        main->>+parallel_queue_1: Run ModuleRun B                                 
        parallel_queue_1-->>-main: Module B installed
    and Other modules installation
        main->>+parallel_queue_2..19: Run ModuleRun C, ...
        parallel_queue_2..19-->>-main: Modules C, ... installed
    end
    
    main-->>-Deckhouse-controller: ParallelModuleRun executed / <br/>Unblock queue
    