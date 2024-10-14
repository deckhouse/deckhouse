/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go/ast"
	"go/parser"
	"go/token"

	"gopkg.in/yaml.v2"
)

const (
	releaseFile         = "release.yaml"
	checkFunctionName   = "RegisterCheck"
	generalDecls        = "general"
	requirementsPackage = "requirements"
)

var (
	moduleRequirementsRegex = regexp.MustCompile(`^(ee\/)?(be\/|fe\/)?modules\/.+\/requirements\/.+\.go$`)
	modulesDirs             = []string{"./modules", "./ee"}
)

type releaseSettings struct {
	Requirements map[string]string `yaml:"requirements"`
}

func RunReleaseRequirementsValidation(info *DiffInfo) (exitCode int) {
	fmt.Printf("Run 'release requirements' validation ...\n")

	exitCode = 0

	fmt.Printf("Check new and updated lines ... ")
	if len(info.Files) == 0 {
		fmt.Printf("OK, diff is empty\n")
	} else {
		fmt.Println("")

		msgs := NewMessages()

		var (
			allRequirements map[string]struct{}
			newRequirements map[string]struct{}
			err             error
		)

		// Checking for changes in release.yaml
		for _, fileInfo := range info.Files {
			if fileInfo.NewFileName == releaseFile && (fileInfo.IsAdded() || fileInfo.IsModified()) && len(fileInfo.NewLines()) != 0 {
				fmt.Println("Gettings new requirements")
				// Get all current requirements from release.yaml
				allRequirements, newRequirements, err = getRequirements(fileInfo.NewLines(), releaseFile)
				if err != nil {
					fmt.Printf("Couldn't get the list of requirements to check: %s\n", err)
					return 1
				}
				break
			}
		}

		if len(newRequirements) == 0 {
			fmt.Println("No new requirements were introduced")
			// there were no changes in the release.yaml file but we still need all release requirements
			if len(allRequirements) == 0 {
				allRequirements, _, err = getRequirements([]string{}, releaseFile)
				if err != nil {
					fmt.Printf("Couldn't get the list of requirements to check: %s\n", err)
					return 1
				}
			}
		} else {
			fmt.Print("New requirements found: ")
			for k, _ := range newRequirements {
				fmt.Printf("%s ", k)
			}
			fmt.Println("")

			// Checking for changes in other */requirements/*.go files
			for _, fileInfo := range info.Files {
				if !fileInfo.HasContent() {
					continue
				}
				// Check only added or modified files
				if !(fileInfo.IsAdded() || fileInfo.IsModified()) {
					continue
				}

				fileName := fileInfo.NewFileName

				// Skip files unrelated to requirements
				if !moduleRequirementsRegex.MatchString(fileName) {
					continue
				}

				// Skip tests
				if strings.HasSuffix(fileName, "_test.go") {
					continue
				}

				// Get added or modified lines
				newLines := fileInfo.NewLines()
				if len(newLines) == 0 {
					msgs.Add(NewSkip(fileName, "no lines added"))
					continue
				}

				// Check if new requirements and checks are introduced in a single PR
				fmt.Println("Checking file: ", fileInfo.NewFileName)
				prematureChecks, _, err := checksAndRequirements(newRequirements, fileName, requirementsPackage)
				if err != nil {
					msgs.Add(NewError(fileName, "coudn't linter due to some errors", err.Error()))
					continue
				}

				if len(prematureChecks) > 0 {
					msgs.Add(NewError(fileName, "should not check release requirements introduced in the same PR", strings.Join(prematureChecks, ", ")))
					continue
				}

				msgs.Add(NewOK(fileName))
			}
		}

		fmt.Println("Inspecting if there are any orphaned requirements (not matching any check) in the release.yaml file")

		allChecks, err := getAllChecks(modulesDirs)
		if err != nil {
			fmt.Printf("Couldn't inspect modules' checks: %s\n", err)
			return 1
		}
		fmt.Println("Following checks have been found:", allChecks)

		for _, check := range allChecks {
			delete(allRequirements, check)
		}

		if len(allRequirements) > 0 {
			output := []string{}
			for requirement, _ := range allRequirements {
				output = append(output, requirement)
			}
			msgs.Add(NewError("release.yaml", "found requirements for non-existent module checks, please review release requirements in release.yaml", strings.Join(output, ", ")))
		}

		msgs.PrintReport()

		if msgs.CountErrors() > 0 {
			exitCode = 1
		}
	}

	return exitCode
}

// checksAndRequirements verifies if there are premature checks (checks that were introduced with the related requirements simultaneously).
// it also checks if there are requirements for non-existent checks (orphaned requirements) in the release.yaml file
func checksAndRequirements(newRequirements map[string]struct{}, fileName, packageName string) ( /* premature checks */ []string /* eligible checks */, []string, error) {
	var (
		prematureChecks []string
		eligibleChecks  []string
		stack           []ast.Node
		errMsgs         []string
	)
	decls := make(map[string]map[string]string)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fileName, nil, 0)
	if err != nil {
		return nil, nil, err
	}

	if file.Name.Name != packageName {
		return nil, nil, nil
	}

	// node is either an assignment or a definition
	ast.Inspect(file, func(n ast.Node) bool {
		parentFunction := findParentFunction(stack)
		if decls[parentFunction] == nil {
			decls[parentFunction] = make(map[string]string)
		}

		switch ntype := n.(type) {
		case *ast.AssignStmt:
			if ntype.Tok == token.ASSIGN || ntype.Tok == token.DEFINE {
			loop:
				for i, lh := range ntype.Lhs {
					var key, value string
					if id, ok := lh.(*ast.Ident); ok {
						key = id.Name
					}

					switch v := ntype.Rhs[i].(type) {
					case *ast.BasicLit:
						value = v.Value

					case *ast.Ident:
						var ok bool
						value, ok = decls[parentFunction][v.Name]
						if !ok {
							value, _ = decls[generalDecls][v.Name]
						}

					default:
						break loop
					}

					if len(key) != 0 && len(value) != 0 {
						decls[parentFunction][key] = strings.Trim(value, "\"")
					}
				}
			}
		// node is part of general declarations
		case *ast.GenDecl:
			if ntype.Tok == token.CONST || ntype.Tok == token.VAR {
				for _, cDecl := range ntype.Specs {
					if vSpec, ok := cDecl.(*ast.ValueSpec); ok {
						for i := 0; i < len(vSpec.Names); i++ {
							var value string
							if len(vSpec.Values) >= i+1 {
								switch v := vSpec.Values[i].(type) {
								case *ast.BasicLit:
									value = strings.Trim(v.Value, "\"")

								case *ast.Ident:
									var ok bool
									value, ok = decls[parentFunction][v.Name]
									if !ok {
										value, _ = decls[generalDecls][v.Name]
									}
								}
							}
							decls[parentFunction][vSpec.Names[i].Name] = value
						}
					}
				}
			}

		// node is a function call
		case *ast.CallExpr:
			if fun, ok := ntype.Fun.(*ast.SelectorExpr); ok {
				// function name is what we are looking for
				if fun.Sel.Name == checkFunctionName {
					switch x := ntype.Args[0].(type) {
					// the function's argument is a string
					case *ast.BasicLit:
						val := strings.Trim(x.Value, "\"")
						// check for a premature check
						if _, found := newRequirements[val]; found {
							prematureChecks = append(prematureChecks, val)
						} else {
							eligibleChecks = append(eligibleChecks, val)
						}

					// the function's argument is a variable
					case *ast.Ident:
						val, ok := decls[parentFunction][x.Name]
						if !ok {
							val, ok = decls[generalDecls][x.Name]
						}

						if ok {
							// check for a premature check
							if _, found := newRequirements[val]; found {
								prematureChecks = append(prematureChecks, val)
							} else {
								eligibleChecks = append(eligibleChecks, val)
							}
						} else {
							errMsgs = append(errMsgs, fmt.Sprintf("Couldn't find declaration of the '%s' variable", x.Name))
						}
					}
				}
			}
		}

		if n == nil {
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, n)
		}

		return true
	})

	if len(errMsgs) > 0 {
		err = fmt.Errorf(strings.Join(errMsgs, ", "))
	}

	return prematureChecks, eligibleChecks, err
}

// Traverses through a "stack" of ast.Nodes to find which function current context belongs to
func findParentFunction(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		fn, ok := stack[i].(*ast.FuncDecl)
		if ok {
			return fn.Name.Name
		}
	}
	return generalDecls
}

// Forms two maps of release requirements, all and new ones (that were indroduced by current PR)
func getRequirements(newlines []string, releaseFile string) ( /* all requirements */ map[string]struct{} /*new requirements*/, map[string]struct{}, error) {
	fileContent, err := os.ReadFile(releaseFile)
	if err != nil {
		return nil, nil, err
	}

	var releaseSettings releaseSettings

	err = yaml.Unmarshal(fileContent, &releaseSettings)
	if err != nil {
		return nil, nil, err
	}

	allRequirements := make(map[string]struct{})
	newRequirements := make(map[string]struct{})

	for requirement, _ := range releaseSettings.Requirements {
		allRequirements[requirement] = struct{}{}
		requirementRegex := regexp.MustCompile(fmt.Sprintf("^  \"%s\":", requirement))
		for _, line := range newlines {
			if requirementRegex.MatchString(line) {
				newRequirements[requirement] = struct{}{}
				break
			}
		}
	}

	return allRequirements, newRequirements, nil
}

// Walks over */requirements/*.go modules' files to inspect if some release requirements have no related checks
func getAllChecks(roots []string) ([]string, error) {
	allChecks := make([]string, 0)

	for _, root := range roots {

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if filepath.Base(filepath.Dir(path)) == "requirements" && !strings.HasSuffix(info.Name(), "_test.go") {
				fmt.Println("Collecting checks from ", path)
				_, checks, err := checksAndRequirements(map[string]struct{}{}, path, requirementsPackage)
				if err != nil {
					return err
				}

				if len(checks) > 0 {
					allChecks = append(allChecks, checks...)
				}
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	slices.Sort(allChecks)

	return slices.Compact(allChecks), nil
}
