package module

import "github.com/flant/addon-operator/pkg/module_manager/go_hook"

func GetHTTPSMode(moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".https.mode"
		globalPath = "global.modules.https.mode"
	)

	v, ok := input.Values.GetOk(modulePath)
	if ok {
		return v.String()
	}

	v, ok = input.Values.GetOk(globalPath)
	if ok {
		return v.String()
	}

	panic("https mode is not defined")
}
