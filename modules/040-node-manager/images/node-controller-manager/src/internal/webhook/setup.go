package webhook

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupWebhooks registers all webhooks with the manager
func SetupWebhooks(mgr ctrl.Manager) error {
	// Setup validation webhook
	if err := SetupNodeGroupValidator(mgr); err != nil {
		return err
	}

	// Setup conversion webhook
	if err := SetupConversionWebhook(mgr); err != nil {
		return err
	}

	return nil
}
