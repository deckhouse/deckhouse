package module

import "github.com/flant/addon-operator/pkg/module_manager/go_hook"

func GetHTTPSMode(moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".https.mode"
		globalPath = "global.modules.https.mode"
	)

	if input.Values.Values.ExistsP(modulePath) {
		return input.Values.Values.Path(modulePath).Data().(string)
	}

	if input.Values.Values.ExistsP(globalPath) {
		return input.Values.Values.Path(globalPath).Data().(string)
	}

	panic("https mode is not defined")
}
