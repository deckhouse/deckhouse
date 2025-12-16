# Registry

This package provides shared models and methods for the registry module.


## Go generate

### Layout

```bash
hack/
  boilerplate.go.txt - header template for generated files
  generate.go — triggers code generation (via go:generate annotation) for all models
models/ — contains data structures for which deepcopy generation is applied
  moduleconfig/
    deckhouse.go — type definitions annotated with `+k8s:deepcopy-gen=true` for automatic method generation
    zz_generated.deepcopy.go — auto-generated file with DeepCopy() methods (created after running go generate)
```

### Example of adding deepCopy generation

1. Create a model in the `./models` folder. Example:
    ```bash
    touch ./models/someconfig/config.go
    ```

    ```go
    package someconfig

    // +k8s:deepcopy-gen=true
    type Config struct {
      User User
    }

    // +k8s:deepcopy-gen=true
    type User struct {
      Username string
      Password string
    }
    ```

2. Run the generation:
    ```bash
    cd ./go_lib/registry
    task generate
    ```

3. Verify the presence of the `zz_generated.deepcopy.go` file with filled deepcopy methods and the inserted header:
    ```bash
    cat ./models/someconfig/zz_generated.deepcopy.go
    ```
