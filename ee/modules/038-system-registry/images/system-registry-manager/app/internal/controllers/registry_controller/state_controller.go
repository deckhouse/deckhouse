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
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

	ReprocessAllNodes func(ctx context.Context) error

	eventRecorder record.EventRecorder

	UserRO   *state.User
	UserRW   *state.User
	PKIState *state.PKIState
	StateOK  bool
}

func (sc *stateController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if sc.ReprocessAllNodes == nil {
		return fmt.Errorf("please set ReprocessAllNodes field")
	}

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

		return name == state.PKISecretName || name == state.UserROSecretName || name == state.UserRWSecretName
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

	var changed, skipReprocess bool

	if changed, err = sc.ensurePKI(ctx, &sc.PKIState); err != nil {
		if errors.Is(err, errorPKIInvalid) {
			log.Error(err, "PKI is invalid and cannot be restored from internal state")

			sc.logModuleWarning(
				nil,
				"PKIFatal",
				"PKI is invalid and cannot be restored from internal state",
			)

			sc.StateOK = false

			err = nil
			return
		}

		err = fmt.Errorf("cannot ensure PKI: %w", err)
		return
	}

	skipReprocess = skipReprocess || changed

	if changed, err = sc.ensureUserSecret(
		ctx,
		state.UserROSecretName,
		&sc.UserRO,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserROSecretName, err)
		return
	}
	skipReprocess = skipReprocess || changed

	if changed, err = sc.ensureUserSecret(
		ctx,
		state.UserRWSecretName,
		&sc.UserRW,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserRWSecretName, err)
		return
	}
	skipReprocess = skipReprocess || changed

	if !sc.StateOK {
		sc.StateOK = true

		sc.logModuleInfo(
			&log,
			"NoError",
			"Module global state is OK",
		)
	}

	if !skipReprocess {
		err = sc.ReprocessAllNodes(ctx)
		if err != nil {
			err = fmt.Errorf("cannot reprocess all nodes: %w", err)
		}
	}

	return
}

func (sc *stateController) ensureUserSecret(ctx context.Context, name string, currentUser **state.User) (bool, error) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureUserSecret", "name", name)

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: sc.Namespace,
	}

	err := sc.Client.Get(ctx, key, &secret)

	if client.IgnoreNotFound(err) != nil {
		return false, fmt.Errorf("cannot get secret %v k8s object: %w", name, err)
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	secretOrig := secret.DeepCopy()

	var actualUser state.User
	notFound := false

	// Not found
	if err != nil {
		notFound = true
	} else {
		actualUser.UserName = string(secret.Data["name"])
		actualUser.Password = string(secret.Data["password"])
		actualUser.HashedPassword = string(secret.Data["passwordHash"])
	}

	if notFound || !actualUser.IsValid() {
		if *currentUser != nil && (*currentUser).IsValid() {
			if notFound {
				sc.logModuleWarning(
					&log,
					fmt.Sprintf("NotFoundUserSecretRestored: %v", name),
					"Secret for user not found, will restore from controller's internal state",
				)
			} else {
				sc.logModuleWarning(
					&log,
					fmt.Sprintf("InvidUserSecretRestored: %v", name),
					"Secret for user invalid, will restore from controller's internal state",
				)
			}

			actualUser = **currentUser
		} else {
			sc.logModuleWarning(
				&log,
				fmt.Sprintf("NewUserSecretGenerated: %v", name),
				"User is invalid, generating new",
			)

			actualUser.UserName = name
			actualUser.GenerateNewPassword()
		}
	}

	if !actualUser.IsPasswordHashValid() {
		actualUser.UpdatePasswordHash()

		sc.logModuleWarning(
			&log,
			fmt.Sprintf("UserPasswordHashUpdated: %v", name),
			"Password hash updated to correspond password",
		)
	}

	// Set labels
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}

	secret.Labels[state.LabelModuleKey] = state.RegistryModuleName
	secret.Labels[state.LabelHeritageKey] = state.LabelHeritageValue
	secret.Labels[state.LabelManagedBy] = state.RegistryModuleName
	secret.Labels[state.LabelTypeKey] = state.UserSecretTypeLabel

	// Set data
	secret.Data = map[string][]byte{
		"name":         []byte(actualUser.UserName),
		"password":     []byte(actualUser.Password),
		"passwordHash": []byte(actualUser.HashedPassword),
	}

	changed := false
	if notFound {
		secret.Name = key.Name
		secret.Namespace = key.Namespace
		secret.Type = state.UserSecretType

		if err = sc.Client.Create(ctx, &secret); err != nil {
			return false, fmt.Errorf("cannot create k8s object: %w", err)
		}

		changed = true
		log.Info("New secret was created")
	} else {
		// Check than we're need to update secret
		if !reflect.DeepEqual(secretOrig, secret) {
			if err = sc.Client.Update(ctx, &secret); err != nil {
				return false, fmt.Errorf("cannot update k8s object: %w", err)
			}

			if secretOrig.ResourceVersion != secret.ResourceVersion {
				log.Info("Secret was updated")
				changed = true
			}

		}
	}

	// Save actual value
	*currentUser = &actualUser

	return changed, nil
}

func (sc *stateController) ensurePKI(ctx context.Context, currentState **state.PKIState) (bool, error) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsurePKI")

	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      state.PKISecretName,
		Namespace: sc.Namespace,
	}

	err := sc.Client.Get(ctx, key, &secret)

	if client.IgnoreNotFound(err) != nil {
		return false, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	// Making a copy unconditionally is a bit wasteful, since we don't
	// always need to update the service. But, making an unconditional
	// copy makes the code much easier to follow, and we have a GC for
	// a reason.
	secretOrig := secret.DeepCopy()

	var actualState state.PKIState
	notFound := false
	isValid := true

	if err != nil {
		notFound = true
	} else {
		caPKI, err := state.DecodeCertKeyFromSecret(
			state.CACertSecretField,
			state.CAKeySecretField,
			&secret,
		)

		if err != nil {
			log.Error(err, "Cannot decode CA PKI")

			sc.logModuleWarning(
				&log,
				"PKICADecodeError",
				fmt.Sprintf("PKI CA decode error: %v", err),
			)

			isValid = false
		} else {
			actualState.CA = &caPKI
		}

		if isValid {
			tokenPKI, err := state.DecodeCertKeyFromSecret(
				state.TokenCertSecretField,
				state.TokenKeySecretField,
				&secret,
			)

			if err != nil {
				log.Error(err, "Cannot decode Token PKI")

				sc.logModuleWarning(
					&log,
					"PKITokenDecodeError",
					fmt.Sprintf("PKI Token decode error: %v", err),
				)

				isValid = false
			} else {
				actualState.Token = &tokenPKI
			}
		}

		if isValid {
			err = pki.ValidateCertWithCAChain(actualState.Token.Cert, actualState.CA.Cert)
			if err != nil {
				log.Error(err, "Token certificate validation error")

				sc.logModuleWarning(
					&log,
					"PKITokenCertValidationError",
					fmt.Sprintf("PKI Token certificate validation error: %v", err),
				)

				isValid = false
			}
		}
	}

	if notFound || !isValid {
		if currentState != nil && *currentState != nil {
			if notFound {
				sc.logModuleWarning(
					&log,
					"PKINotfoundRestored",
					"PKI secret not found, will restore from controller's internal state",
				)
			} else {
				sc.logModuleWarning(
					&log,
					"PKIInvalidRestored",
					"PKI secret invalid, so restored from controller's internal state",
				)
			}

			actualState = **currentState
		} else {
			// PKI is invalid and we don't have some to restore
			return false, errorPKIInvalid
		}
	}

	// Set labels
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}

	secret.Labels[state.LabelModuleKey] = state.RegistryModuleName
	secret.Labels[state.LabelHeritageKey] = state.LabelHeritageValue
	secret.Labels[state.LabelManagedBy] = state.RegistryModuleName
	secret.Labels[state.LabelTypeKey] = state.CASecretTypeLabel

	secret.Data = make(map[string][]byte)
	if err = state.EncodeCertKeyToSecret(
		*actualState.CA,
		state.CACertSecretField,
		state.CAKeySecretField,
		&secret,
	); err != nil {
		return false, fmt.Errorf("cannot encode CA PKI to secret: %w", err)
	}

	if err = state.EncodeCertKeyToSecret(
		*actualState.Token,
		state.TokenCertSecretField,
		state.TokenKeySecretField,
		&secret,
	); err != nil {
		return false, fmt.Errorf("cannot encode Token PKI to secret: %w", err)
	}

	changed := false
	if notFound {
		secret.Name = key.Name
		secret.Namespace = key.Namespace
		secret.Type = state.CASecretType

		if err = sc.Client.Create(ctx, &secret); err != nil {
			return false, fmt.Errorf("cannot create k8s object: %w", err)
		}

		changed = true
		log.Info("New secret was created")
	} else {
		// Check than we're need to update secret
		if !reflect.DeepEqual(secretOrig, secret) {
			if err = sc.Client.Update(ctx, &secret); err != nil {
				return false, fmt.Errorf("cannot update k8s object: %w", err)
			}

			if secretOrig.ResourceVersion != secret.ResourceVersion {
				log.Info("Secret was updated")
				changed = true
			}

		}
	}

	// Save actual value
	*currentState = &actualState

	return changed, nil
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
