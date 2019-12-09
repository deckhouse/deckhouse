Matrix Tests
============

Matrix tests are employed to validate all variations of values.yaml file to render helm chart.

### Usage

Create the file named values_matrix_test.yaml in your chart directory.

Example for 1x1 matrix (values_matrix_test.yaml):
```yaml
replicas: 1
image: registry.example.com
environment:
- test
- prod
```

To create increase your matrix dimensional you should use special functions:

#### Choices

Choices allow splitting values.yaml.

* `__ConstantChoise__` - array of items you split values.yaml with.

Example values_matrix_test.yaml:
```yaml
replicas: 
  __ConstantChoise__: [1, 2]
image: registry.example.com
environment:
- test
- prod
```

It creates two different values.yaml files for tests:
```yaml
replicas: 1
image: registry.example.com
environment:
- test
- prod
```
```yaml
replicas: 2
image: registry.example.com
environment:
- test
- prod
```

You can add additional choice to create more variants:

Example values_matrix_test.yaml:
```yaml
replicas: 
  __ConstantChoise__: [1, 2]
image: registry.example.com
environment:
  __ConstantChoise__:
  - ["test", "prod"]
  - ["dev", "stage"]
```

It creates four values.yaml files for tests:
```yaml
replicas: 1
image: registry.example.com
environment:
- test
- prod
```
```yaml
replicas: 2
image: registry.example.com
environment:
- test
- prod
```
```yaml
replicas: 1
image: registry.example.com
environment:
- dev
- stage
```
```yaml
replicas: 2
image: registry.example.com
environment:
- dev
- stage
```
And so on.

#### Items

Special values.yaml that can be used in values_matrix_test.yaml.

* `__EmptyItem__` - completely delete object

Example values_matrix_test.yaml:
```yaml
replicas: 1
image: 
  __ConstantChoise__: [registry.example.com, __EmptyItem__]
```
Generated values.yaml:
```yaml
replicas: 1
image: registry.example.com
```
```yaml
replicas: 1
```
