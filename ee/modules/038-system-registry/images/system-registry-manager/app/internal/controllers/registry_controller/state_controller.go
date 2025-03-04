/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"embeded-registry-manager/internal/state"
	"embeded-registry-manager/internal/utils/pki"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type StateController = stateController

var _ reconcile.Reconciler = &stateController{}

var errorPKIInvalid = errors.New("invalid PKI found and internal state is not populated")

type stateController struct {
	Client    client.Client
	Namespace string

	eventRecorder record.EventRecorder

	userRO           *state.User
	userRW           *state.User
	userMirrorPuller *state.User
	userMirrorPusher *state.User

	globalPKI *state.GlobalPKI

	stateOK bool
}

func (sc *stateController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	controllerName := "global-state-controller"

	sc.eventRecorder = mgr.GetEventRecorderFor(controllerName)

	moduleConfig := state.GetModuleConfigObject()

	moduleConfigPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == state.RegistryModuleName
	})

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != sc.Namespace {
			return false
		}

		name := obj.GetName()
		return name == state.PKISecretName ||
			name == state.GlobalSecretsName ||
			name == state.UserROSecretName ||
			name == state.UserRWSecretName ||
			name == state.UserMirrorPullerName ||
			name == state.UserMirrorPusherName
	})

	secretsHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx)

		log.Info(
			"Secret changed, will trigger reconcile",
			"secret", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"controller", controllerName,
		)

		var req reconcile.Request
		req.Name = state.RegistryModuleName

		return []reconcile.Request{req}
	})

	err := ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&moduleConfig, builder.WithPredicates(moduleConfigPredicate)).
		Watches(
			&corev1.Secret{},
			secretsHandler,
			builder.WithPredicates(secretsPredicate),
		).
		Complete(sc)

	if err != nil {
		return fmt.Errorf("cannot build controller: %w", err)
	}

	return nil
}

func (sc *stateController) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	log := ctrl.LoggerFrom(ctx)

	config, err := state.LoadModuleConfig(ctx, sc.Client)
	if err != nil {
		err = fmt.Errorf("cannot load module config: %w", err)
		return
	}

	if !config.Enabled {
		log.Info("Module disabled will not reconcile other objects")

		return
	}

	if err = sc.ensurePKI(ctx, &sc.globalPKI); err != nil {
		if errors.Is(err, errorPKIInvalid) {
			log.Error(err, "PKI is invalid and cannot be restored from internal state")

			sc.logModuleWarning(
				nil,
				"PKIFatal",
				"PKI is invalid and cannot be restored from internal state",
			)

			sc.stateOK = false

			err = nil
			return
		}

		err = fmt.Errorf("cannot ensure PKI: %w", err)
		return
	}

	if err = sc.ensureUserSecret(
		ctx,
		state.UserROSecretName,
		&sc.userRO,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserROSecretName, err)
		return
	}

	if err = sc.ensureUserSecret(
		ctx,
		state.UserRWSecretName,
		&sc.userRW,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserRWSecretName, err)
		return
	}

	if err = sc.ensureUserSecret(
		ctx,
		state.UserMirrorPullerName,
		&sc.userMirrorPuller,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserMirrorPullerName, err)
		return
	}

	if err = sc.ensureUserSecret(
		ctx,
		state.UserMirrorPusherName,
		&sc.userMirrorPusher,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserMirrorPusherName, err)
		return
	}

	if err = sc.ensureGlobalSecrets(ctx); err != nil {
		err = fmt.Errorf("cannot ensure global secrets: %w", err)
		return
	}

	if !sc.stateOK {
		sc.stateOK = true

		sc.logModuleInfo(
			&log,
			"NoError",
			"Module global state is OK",
		)
	}

	return
}

func (sc *stateController) ensureGlobalSecrets(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureGlobalSecrets")

	var actualValue state.GlobalSecrets

	updated, err := ensureSecret(
		ctx,
		sc.Client,
		state.GlobalSecretsName,
		sc.Namespace,
		func(ctx context.Context, secret *corev1.Secret, found bool) error {
			valid := true
			if found {
				if err := actualValue.DecodeSecret(secret); err != nil {
					sc.logModuleWarning(
						&log,
						"GlobalSecretsDecodeError",
						fmt.Sprintf("Cannot decode global secrets: %v", err),
					)
					valid = false
				} else if err = actualValue.Validate(); err != nil {
					sc.logModuleWarning(
						&log,
						"GlobalSecretsValidationError",
						fmt.Sprintf("Global secrets validation error: %v", err),
					)
					valid = false
				}
			}

			if !found || !valid {
				sc.logModuleWarning(
					&log,
					"GlobalSecretsGenerateNew",
					"Global secrets is invalid, generating new",
				)

				if randomValue, err := pki.GenerateRandomSecret(); err != nil {
					return fmt.Errorf("cannot generate HTTP secret: %w", err)
				} else {
					actualValue.HttpSecret = randomValue
				}
			}

			if err := actualValue.EncodeSecret(secret); err != nil {
				return fmt.Errorf("cannot encode to secret: %w", err)
			}
			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("cannot ensure secret: %w", err)
	}
	if updated {
		log.Info("Secret was updated")
	}

	return nil
}

func (sc *stateController) ensureUserSecret(ctx context.Context, name string, currentValue **state.User) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureUserSecret", "name", name)

	var actualValue state.User

	updated, err := ensureSecret(
		ctx,
		sc.Client,
		name,
		sc.Namespace,
		func(ctx context.Context, secret *corev1.Secret, found bool) error {
			valid := true
			if found {
				if err := actualValue.DecodeSecret(secret); err != nil {
					sc.logModuleWarning(
						&log,
						fmt.Sprintf("UserDecodeError: %v", name),
						fmt.Sprintf("Cannot decode user data from secret: %v", err),
					)

					valid = false
				} else {
					valid = actualValue.IsValid()
				}
			}

			if !found || !valid {
				if currentValue != nil && *currentValue != nil {
					sc.logModuleWarning(
						&log,
						fmt.Sprintf("UserSecretRestored: %v", name),
						"Secret for user is invalid, restoring from controller's internal state",
					)

					actualValue = **currentValue
				} else {
					sc.logModuleWarning(
						&log,
						fmt.Sprintf("UserSecretGenerateNew: %v", name),
						"User is invalid, generating new",
					)

					actualValue.UserName = name
					actualValue.GenerateNewPassword()
				}
			}

			if !actualValue.IsPasswordHashValid() {
				actualValue.UpdatePasswordHash()

				sc.logModuleWarning(
					&log,
					fmt.Sprintf("UserPasswordHashUpdated: %v", name),
					"Password hash updated to correspond password",
				)
			}

			if err := actualValue.EncodeSecret(secret); err != nil {
				return fmt.Errorf("cannot encode to secret: %w", err)
			}
			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("cannot ensure secret: %w", err)
	}
	if updated {
		log.Info("Secret was updated")
	}

	// Save actual value
	*currentValue = &actualValue
	return nil
}

func (sc *stateController) ensurePKI(ctx context.Context, currentValue **state.GlobalPKI) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsurePKI")

	var actualValue state.GlobalPKI

	updated, err := ensureSecret(
		ctx,
		sc.Client,
		state.PKISecretName,
		sc.Namespace,
		func(ctx context.Context, secret *corev1.Secret, found bool) error {
			valid := true
			if found {
				if err := actualValue.DecodeSecret(secret); err != nil {
					sc.logModuleWarning(
						&log,
						"PKIDecodeError",
						fmt.Sprintf("PKI decode error: %v", err),
					)
					valid = false
				} else if err = actualValue.Validate(); err != nil {
					sc.logModuleWarning(
						&log,
						"PKIValidationError",
						fmt.Sprintf("PKI validation error: %v", err),
					)
					valid = false
				}
			}

			if !found || !valid {
				if currentValue != nil && *currentValue != nil {
					sc.logModuleWarning(
						&log,
						"PKIInvalidRestored",
						"PKI secret is invalid, restoring from controller's internal state",
					)

					actualValue = **currentValue
				} else {
					// PKI is invalid and we don't have some to restore
					return errorPKIInvalid
				}
			}

			if err := actualValue.EncodeSecret(secret); err != nil {
				return fmt.Errorf("cannot encode to secret: %w", err)
			}
			return nil
		},
	)

	if err != nil {
		return fmt.Errorf("cannot ensure secret: %w", err)
	}
	if updated {
		log.Info("Secret was updated")
	}

	// Save actual value
	*currentValue = &actualValue
	return nil
}

func (sc *stateController) logModuleWarning(log *logr.Logger, reason, message string) {
	obj := state.GetModuleConfigObject()
	obj.SetNamespace(sc.Namespace)

	sc.eventRecorder.Event(&obj, corev1.EventTypeWarning, reason, message)

	if log != nil {
		log.Info(message, "reason", reason)
	}
}

func (sc *stateController) logModuleInfo(log *logr.Logger, reason, message string) {
	obj := state.GetModuleConfigObject()
	obj.SetNamespace(sc.Namespace)

	sc.eventRecorder.Event(&obj, corev1.EventTypeNormal, reason, message)

	if log != nil {
		log.Info(message, "reason", reason)
	}
}
