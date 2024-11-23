/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"context"
	"embeded-registry-manager/internal/state"
	"fmt"

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

type stateController struct {
	Client    client.Client
	Namespace string

	ReprocessAllNodes func(ctx context.Context) error

	eventRecorder record.EventRecorder

	UserRO holder[state.User]
	UserRW holder[state.User]
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

		return name == state.RegistryPKISecretName || name == state.UserROSecretName || name == state.UserRWSecretName
	})

	secretsHandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx)

		log.Info(
			"Secret was changed, will trigger reconcile",
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

	log.Info("--Reconcile--")

	config, err := state.LoadModuleConfig(ctx, sc.Client)
	if err != nil {
		err = fmt.Errorf("cannot load module config: %w", err)
		return
	}

	log.Info("Got Module Config", "config", config)

	if !config.Enabled {
		log.Info("Module disabled will not reconcile other objects")
		return
	}

	if err = sc.EnsureUserSecret(
		ctx,
		state.UserROSecretName,
		&sc.UserRO,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserROSecretName, err)
		return
	}

	if err = sc.EnsureUserSecret(
		ctx,
		state.UserRWSecretName,
		&sc.UserRW,
	); err != nil {
		err = fmt.Errorf("cannot ensure secret %v for user: %w", state.UserRWSecretName, err)
		return
	}

	err = sc.ReprocessAllNodes(ctx)
	if err != nil {
		err = fmt.Errorf("cannot reprocess all nodes: %w", err)
	}

	return
}

func (sc *stateController) EnsureUserSecret(ctx context.Context, name string, user *holder[state.User]) error {
	log := ctrl.LoggerFrom(ctx).
		WithValues("action", "EnsureUserSecret", "name", name)

	secret := corev1.Secret{}

	key := types.NamespacedName{
		Name:      name,
		Namespace: sc.Namespace,
	}

	err := sc.Client.Get(ctx, key, &secret)

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("cannot get secret %v k8s object: %w", name, err)
	}

	var ret state.User
	notFound := false

	// Not found
	if err != nil {
		notFound = true
	} else {
		ret.UserName = string(secret.Data["name"])
		ret.Password = string(secret.Data["password"])
		ret.HashedPassword = string(secret.Data["passwordHash"])
	}

	if notFound || !ret.IsValid() {
		if user.Value != nil && user.Value.IsValid() {
			if notFound {
				log.Info("Secret for user not found, will restore from memory")
			} else {
				log.Info("Secret for user is invalid, will restore from memory")
			}

			ret = *user.Value
		} else {
			log.Info("User is invalid, generating new")

			ret.UserName = name
			ret.GenerateNewPassword()
		}
	}

	if !ret.IsPasswordHashValid() {
		ret.UpdatePasswordHash()

		log.Info("Password hash for user not corresponds password, updating")
	}

	// Set labels
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}

	secret.Labels[state.LabelModuleKey] = state.RegistryModuleName
	secret.Labels[state.LabelHeritageKey] = state.LabelHeritageValue
	secret.Labels[state.LabelManagedBy] = state.RegistryModuleName

	// Set data
	secret.Data = map[string][]byte{
		"name":         []byte(ret.UserName),
		"password":     []byte(ret.Password),
		"passwordHash": []byte(ret.HashedPassword),
	}

	if notFound {
		secret.Name = key.Name
		secret.Namespace = key.Namespace
		secret.Type = state.UserSecretType

		if err = sc.Client.Create(ctx, &secret); err != nil {
			return fmt.Errorf("cannot create k8s object: %w", err)
		}

		log.Info("New secret for user was created")
	} else {
		currentVersion := secret.ResourceVersion

		if err = sc.Client.Update(ctx, &secret); err != nil {
			return fmt.Errorf("cannot update k8s object: %w", err)
		}

		if currentVersion != secret.ResourceVersion {
			log.Info("Secret for user was updated")
		}
	}

	// Set actual user value
	user.Value = &ret

	return nil
}
