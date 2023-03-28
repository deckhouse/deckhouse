package debug

import (
	"fmt"
	"net/http"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	sh_app "github.com/flant/shell-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
	"github.com/go-chi/chi/v5"
	"gopkg.in/alecthomas/kingpin.v2"

	deckhouse_config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

func RegisterModuleEnableRoutes(dbgSrv *sh_debug.Server, op *addon_operator.AddonOperator) {
	dbgSrv.RoutePOST("/module/{name}/{action:(disable|enable)}", func(r *http.Request) (interface{}, error) {
		modName := chi.URLParam(r, "name")
		if modName == "" {
			return nil, fmt.Errorf("'name' parameter is required")
		}

		action := chi.URLParam(r, "action")
		switch action {
		case "enable":
			err := deckhouse_config.SetModuleConfigEnabledFlag(op.KubeClient, modName, true)
			if err != nil {
				return nil, fmt.Errorf("Enable module failed: %v", err)
			}
			return "Module enabled", nil
		case "disable":
			err := deckhouse_config.SetModuleConfigEnabledFlag(op.KubeClient, modName, true)
			if err != nil {
				return nil, fmt.Errorf("Disable module failed: %v", err)
			}
			return "Module disabled", nil
		}
		return nil, fmt.Errorf("Unknown action '%s' for module '%s'", action, modName)
	})
}

func DefineModuleConfigDebugCommands(kpApp *kingpin.Application) {
	moduleCmd := kpApp.GetCommand("module")

	var moduleName string
	moduleEnableCmd := moduleCmd.Command("enable", "Enable module via spec.enabled flag in ModuleConfig resource.").
		Action(func(c *kingpin.ParseContext) error {
			response, err := ModuleEnable(sh_debug.DefaultClient(), moduleName)
			if err != nil {
				return err
			}
			fmt.Println(string(response))
			return nil
		})
	moduleEnableCmd.Arg("module_name", "").Required().StringVar(&moduleName)
	// --debug-unix-socket <file>
	sh_app.DefineDebugUnixSocketFlag(moduleEnableCmd)

	moduleDisableCmd := moduleCmd.Command("disable", "Enable module via spec.enabled flag in ModuleConfig resource.").
		Action(func(c *kingpin.ParseContext) error {
			response, err := ModuleDisable(sh_debug.DefaultClient(), moduleName)
			if err != nil {
				return err
			}
			fmt.Println(string(response))
			return nil
		})
	moduleDisableCmd.Arg("module_name", "").Required().StringVar(&moduleName)
	// --debug-unix-socket <file>
	sh_app.DefineDebugUnixSocketFlag(moduleEnableCmd)
}

func ModuleEnable(client *sh_debug.Client, modName string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/module/%s/enable", modName)
	return client.Post(url, nil)
}

func ModuleDisable(client *sh_debug.Client, modName string) ([]byte, error) {
	url := fmt.Sprintf("http://unix/module/%s/disable", modName)
	return client.Post(url, nil)
}
