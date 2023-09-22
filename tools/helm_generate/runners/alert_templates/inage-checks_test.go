package alerttemplates

import (
	"fmt"
	"os"
	"testing"
)

// func TestReadUserDefinedValues(t *testing.T) {
// 	err := os.Chdir("/Users/dkoba/Work/deckhouse/tools")
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	deckhouseRoot, err := helper.DeckhouseRoot()
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	val, err := readUserDefinedValues(deckhouseRoot, userDefinedValuesPath)
// 	fmt.Println("val: ", val, "err: ", err)
// }

// func TestRangeOwerModules(t *testing.T) {
// 	err := os.Chdir("/Users/dkoba/Work/deckhouse/tools")
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	modules := rangeOwerModules()
// 	fmt.Println("modules: ", modules)

// 	for module := range modules {
// 		if module.Name == "040-node-manager" {
// 			rangeOwerModuleTemplates(module.Path)
// 		}
// 	}
// }

func TestRun(t *testing.T) {
	err := os.Chdir("/Users/dkoba/Work/deckhouse/tools")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(run())
}
