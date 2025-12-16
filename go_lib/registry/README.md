# Registry

This package provides shared models and methods for the registry module.


## Go Generate

### Layout

```bash
hack/ — scripts and helper files for go generate execution
  codegen.sh — library with code generation functions
  update-codegen.sh — runs code generation for the entire package
  verify-codegen.sh — checks for ungenerated or incorrectly generated files
  boilerplate.go.txt — header template for generated files

models/ — folder with models for which deepcopy go generate is applied
  ...
```

### How codegen works

DeepCopy uses k8s code-generator to generate deepcopy methods into: `models/<model-name>/zz_generated.deepcopy.go`.


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
    cd ./hack

    # Go generate
    ./update-codegen.sh

    # Ensure there are no errors after generation
    ./verify-codegen.sh
    ```

3. Verify the presence of the `zz_generated.deepcopy.go` file with filled deepcopy methods and the inserted header:
    ```bash
    cat ./models/someconfig/zz_generated.deepcopy.go
    ```