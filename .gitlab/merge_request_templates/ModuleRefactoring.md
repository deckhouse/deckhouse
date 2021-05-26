## Description
This MR is a part of "Global" Deckhouse refactoring.

## Why we need it and what problem does it solve?
It is aimed to improve UX and DX. 

* Go is more reliable and suitable to describe cloud infrastructure components, especially in distributed systems/environments
* Open API specs will help us with bounding a contract between user and deckhouse, and between hooks and helm
* An Open API spec for a module will become the single source of truth (documenting, validating, testing)
* More tests, more stability

## Checklist
<!---
  Please click the checkbox if you have already done actions from descriptions.
  
  Remember that you don't have to check all the boxes. You should check them only if it is necessary.
--->
- [ ] Rewrite all shell hooks in Go
- [ ] Cover code with unit tests
- [ ] Add `modules/$MODULE_NAME/openapi` specs and test cases for them
- [ ] Substitute matrix tests by providing `x-examples` fields to previously added specs  
- [ ] Delete `modules/$MODULE_NAME/values.yaml`
- [ ] Delete `modules/$MODULE_NAME/values_matrix_tests.yaml`
- [ ] Execute code-gen script to ensure new hooks registration `go generate deckhouse-controller/register.go`
