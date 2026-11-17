/*
Copyright 2026 Flant JSC

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

package migration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// The dex-authenticator of an object as the module chart rendered it before the shared instances:
// a Deployment with its PodDisruptionBudget, VerticalPodAutoscaler and NetworkPolicy and a Secret
// with the secrets of the object in the namespace of the object, and a copy of the registry
// Secret per namespace. The module hooks keep the release from removing them (see
// hooks/internal/handover of the module): the pods must serve until the instance does. The
// controller removes them once the Service of the object points at the instance. This file is
// that removal and the import of the secrets; see README.md of the package.
//
// The pods of an object rendered while its Dex client had the ID
// <name>-<namespace>-dex-authenticator sign users in with that client, so the hooks keep it too,
// and it goes together with the pods.

const (
	// dexAuthenticatorForLabel names the DexAuthenticator on the objects the chart rendered for it.
	dexAuthenticatorForLabel = "deckhouse.io/dex-authenticator-for"

	legacyApp        = "dex-authenticator"
	legacyHeritage   = "deckhouse"
	legacyModule     = "user-authn"
	legacyNameSuffix = "-dex-authenticator"

	legacySecretPrefix     = "dex-authenticator-"
	legacyClientSecretKey  = "client-secret"
	legacyCookieSecretKey  = "cookie-secret"
	legacyRegistrySecret   = "registry-dex-authenticator"
	legacyCredentialsLabel = "name"
	legacyCredentialsValue = "credentials"

	// formerClientApp is the app label the chart put on the Dex client of a DexAuthenticator.
	formerClientApp = "dex"
)

// legacyKinds are the kinds of the objects of the dex-authenticator of an object, the Secret aside.
var legacyKinds = []schema.GroupVersionKind{
	{Group: "apps", Version: "v1", Kind: "Deployment"},
	{Group: "policy", Version: "v1", Kind: "PodDisruptionBudget"},
	{Group: "autoscaling.k8s.io", Version: "v1", Kind: "VerticalPodAutoscaler"},
	{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"},
}

var secretGVK = schema.GroupVersionKind{Version: "v1", Kind: "Secret"}

// legacyLabels are the labels the chart put on every object of the dex-authenticator of an
// object. Nobody but Deckhouse can write an object with the heritage label.
func legacyLabels() client.MatchingLabels {
	return client.MatchingLabels{"app": legacyApp, "heritage": legacyHeritage, "module": legacyModule}
}

// isLegacy reports whether obj is an object of the dex-authenticator of the DexAuthenticator
// name; an empty name is for the registry Secret, which serves every object of its namespace.
func isLegacy(obj client.Object, name string) bool {
	objLabels := obj.GetLabels()
	for key, value := range legacyLabels() {
		if objLabels[key] != value {
			return false
		}
	}
	return name == "" || objLabels[dexAuthenticatorForLabel] == name
}

// FormerDexAuthenticatorClientKey is where the Dex client of the DexAuthenticator name in namespace
// lay while its ID was LegacyDexAuthenticatorClientID.
func FormerDexAuthenticatorClientKey(name, namespace string) types.NamespacedName {
	return types.NamespacedName{Namespace: naming.DexNamespace, Name: naming.ToFnvLikeDex(LegacyDexAuthenticatorClientID(name, namespace))}
}

// HasFormerDexAuthenticatorClientLabels reports whether obj carries the labels of a Dex client the
// chart rendered for a DexAuthenticator under its former ID and the module hook handed to the
// controller. The client of a DexAuthenticator under the ID of today carries the name of its
// object, and is the business of the reconciler of the objects.
func HasFormerDexAuthenticatorClientLabels(obj client.Object) bool {
	objLabels := obj.GetLabels()
	for key, value := range map[string]string{
		"app": formerClientApp, "heritage": legacyHeritage, "module": legacyModule,
		adoption.ManagedByLabel: adoption.ManagedByValue,
	} {
		if objLabels[key] != value {
			return false
		}
	}
	_, named := objLabels[dexAuthenticatorForLabel]
	return !named
}

// IsFormerDexAuthenticatorClient reports whether obj is the Dex client the chart rendered for the
// DexAuthenticator name in namespace under its former ID, handed to the controller by the module
// hook. The ID alone does not tell: the DexAuthenticator kubeconfig in foo and the kubeconfig of
// the cluster foo-dex-authenticator give one ID, and that client is rendered with another app
// label.
func IsFormerDexAuthenticatorClient(obj *unstructured.Unstructured, name, namespace string) bool {
	if !HasFormerDexAuthenticatorClientLabels(obj) {
		return false
	}
	have, _, _ := unstructured.NestedString(obj.Object, "id")
	return have == LegacyDexAuthenticatorClientID(name, namespace)
}

// legacyObjectName is the name of the Deployment of the dex-authenticator of an object and of
// the objects around it, which is also the name of the Service of the object.
func legacyObjectName(name string) string {
	safe, _, _ := naming.SafeObjectName(name + legacyNameSuffix)
	return safe
}

// legacySecretName is the name of the Secret with the secrets of an object.
func legacySecretName(name string) string {
	safe, _, _ := naming.SafeObjectName(legacySecretPrefix + name)
	return safe
}

// DexAuthenticatorSecrets are the secret of the Dex client of a DexAuthenticator and the secret
// of its cookies.
type DexAuthenticatorSecrets struct {
	Client string
	Cookie string
}

// DexAuthenticatorLegacy knows which DexAuthenticator objects still have a dex-authenticator of
// their own. It lists what is left once, when it is first asked, so that the reconcile of an
// object that has none costs no request.
type DexAuthenticatorLegacy struct {
	client client.Client
	// reader bypasses the cache: nothing of these objects is cached.
	reader client.Reader
	log    logr.Logger

	mu      sync.Mutex
	listed  bool
	pending map[types.NamespacedName]struct{}
	// former are the objects whose Dex client of the former ID is there, whether they have
	// anything else left or not.
	former map[types.NamespacedName]struct{}
}

// NewDexAuthenticatorLegacy returns a DexAuthenticatorLegacy that has not looked at the cluster yet.
func NewDexAuthenticatorLegacy(c client.Client, reader client.Reader, log logr.Logger) *DexAuthenticatorLegacy {
	return &DexAuthenticatorLegacy{
		client: c, reader: reader, log: log,
		pending: map[types.NamespacedName]struct{}{},
		former:  map[types.NamespacedName]struct{}{},
	}
}

// list finds the objects that still have something of a dex-authenticator of their own, and
// removes the registry Secret of a namespace where none has: a start that was interrupted after
// the last object of a namespace lost its dex-authenticator left it there.
func (l *DexAuthenticatorLegacy) list(ctx context.Context) error {
	pending := map[types.NamespacedName]struct{}{}
	var registries []string
	for _, gvk := range append(slices.Clone(legacyKinds), secretGVK) {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		err := l.reader.List(ctx, list, legacyLabels())
		if meta.IsNoMatchError(err) {
			// The cluster has no such kind, so no such object either.
			continue
		}
		if err != nil {
			return fmt.Errorf("list the %s objects of the dex-authenticators: %w", gvk.Kind, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			// The registry Secret names no object: it goes with the last one of its namespace.
			if name := item.GetLabels()[dexAuthenticatorForLabel]; name != "" {
				pending[types.NamespacedName{Namespace: item.GetNamespace(), Name: name}] = struct{}{}
			} else if gvk == secretGVK && item.GetName() == legacyRegistrySecret {
				registries = append(registries, item.GetNamespace())
			}
		}
	}

	for _, namespace := range registries {
		if slices.ContainsFunc(slices.Collect(maps.Keys(pending)), func(key types.NamespacedName) bool { return key.Namespace == namespace }) {
			continue
		}
		if err := l.remove(ctx, secretGVK, types.NamespacedName{Namespace: namespace, Name: legacyRegistrySecret}, ""); err != nil {
			return err
		}
	}

	former, err := l.formerClients(ctx, pending)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending, l.former, l.listed = pending, former, true
	return nil
}

// formerClients finds the objects whose Dex client of the former ID is there, and deletes such a
// client of an object that is gone. The client is looked for by itself, not through what is left
// in the namespace of the object: the Deployment and the rest may be gone already, and the client,
// which the module hook kept from the release, would then stay for ever, a trusted peer of the
// kubernetes client for an object with access to the Kubernetes API.
//
// The former ID does not name one object for sure: the object a in the namespace b-c and the
// object a-b in the namespace c both had a-b-c-dex-authenticator. Such a client is taken for the
// object of those that still has a dex-authenticator of its own (pending), the one whose pods sign
// users in with it; when none or both have, it is left alone, so that serving one object never
// removes the client the pods of another one use.
func (l *DexAuthenticatorLegacy) formerClients(ctx context.Context, pending map[types.NamespacedName]struct{}) (map[types.NamespacedName]struct{}, error) {
	clients := controller.List(controller.OAuth2ClientGVK)
	err := l.reader.List(ctx, clients, client.InNamespace(naming.DexNamespace), client.MatchingLabels{
		"app": formerClientApp, "heritage": legacyHeritage, "module": legacyModule,
		adoption.ManagedByLabel: adoption.ManagedByValue,
	})
	if meta.IsNoMatchError(err) {
		return map[types.NamespacedName]struct{}{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list the Dex clients of the former IDs: %w", err)
	}
	byName := make(map[string]*unstructured.Unstructured, len(clients.Items))
	for i := range clients.Items {
		if item := &clients.Items[i]; HasFormerDexAuthenticatorClientLabels(item) {
			byName[item.GetName()] = item
		}
	}
	former := map[types.NamespacedName]struct{}{}
	if len(byName) == 0 {
		return former, nil
	}

	// Once, at the start, and only when a client of a former ID is there: the names of the
	// DexAuthenticators are all that is read from the list.
	authenticators := controller.List(controller.DexAuthenticatorGVK)
	if err := l.reader.List(ctx, authenticators); err != nil {
		return nil, fmt.Errorf("list DexAuthenticators: %w", err)
	}
	claimants := make(map[string][]types.NamespacedName, len(byName))
	for i := range authenticators.Items {
		a := &authenticators.Items[i]
		name := FormerDexAuthenticatorClientKey(a.GetName(), a.GetNamespace()).Name
		if obj, ok := byName[name]; ok && IsFormerDexAuthenticatorClient(obj, a.GetName(), a.GetNamespace()) {
			claimants[name] = append(claimants[name], client.ObjectKeyFromObject(a))
		}
	}
	for name, keys := range claimants {
		delete(byName, name)
		owner, ok := formerClientOwner(keys, pending)
		if !ok {
			l.log.Info("the Dex client of a former ID is left alone: more than one DexAuthenticator has that ID",
				"name", name, "objects", keys)
			continue
		}
		former[owner] = struct{}{}
	}

	// What is left has the labels of a former client and no object: the object was deleted. Only a
	// client whose ID has the form of a former ID goes; anything else under these labels stays.
	for _, obj := range byName {
		if id, _, _ := unstructured.NestedString(obj.Object, "id"); !strings.HasSuffix(id, legacyNameSuffix) {
			continue
		}
		if err := l.deleteFormerClient(ctx, obj); err != nil {
			return nil, err
		}
	}
	return former, nil
}

// Pending lists the objects that still have something of a dex-authenticator of their own, the
// Dex client of the former ID included. The reconciler of the objects is given each of them at
// the start: an object deleted while the controller was down brings no event, and what is left of
// it would stay.
func (l *DexAuthenticatorLegacy) Pending(ctx context.Context) ([]types.NamespacedName, error) {
	if err := l.list(ctx); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	keys := maps.Clone(l.pending)
	maps.Copy(keys, l.former)
	return slices.Collect(maps.Keys(keys)), nil
}

// Has reports whether the object key may still have a dex-authenticator of its own.
func (l *DexAuthenticatorLegacy) Has(ctx context.Context, key types.NamespacedName) (bool, error) {
	l.mu.Lock()
	listed := l.listed
	l.mu.Unlock()
	if !listed {
		if err := l.list(ctx); err != nil {
			return false, err
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, pending := l.pending[key]
	return pending, nil
}

// done forgets the object key and reports whether no other object of its namespace is left.
func (l *DexAuthenticatorLegacy) done(key types.NamespacedName) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.pending, key)
	for other := range maps.Keys(l.pending) {
		if other.Namespace == key.Namespace {
			return false
		}
	}
	return true
}

// Secrets returns the secrets the dex-authenticator of the object kept in its namespace, and
// false when there is no such Secret or it does not hold both. Only the objects of the d8-*
// namespaces keep their secrets: the Secret of an object of a user was readable to everybody
// who reads Secrets in its namespace, so such an object gets new ones.
func (l *DexAuthenticatorLegacy) Secrets(ctx context.Context, object types.NamespacedName) (DexAuthenticatorSecrets, bool, error) {
	if !naming.IsSystemNamespace(object.Namespace) {
		return DexAuthenticatorSecrets{}, false, nil
	}
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: object.Namespace, Name: legacySecretName(object.Name)}
	err := l.reader.Get(ctx, key, secret)
	if apierrors.IsNotFound(err) {
		return DexAuthenticatorSecrets{}, false, nil
	}
	if err != nil {
		return DexAuthenticatorSecrets{}, false, fmt.Errorf("get Secret %s: %w", key, err)
	}
	if !isLegacy(secret, object.Name) || secret.Labels[legacyCredentialsLabel] != legacyCredentialsValue {
		return DexAuthenticatorSecrets{}, false, nil
	}

	secrets := DexAuthenticatorSecrets{Client: string(secret.Data[legacyClientSecretKey]), Cookie: string(secret.Data[legacyCookieSecretKey])}
	// The proxy takes a cookie secret of 16, 24 or 32 bytes.
	if secrets.Client == "" || !slices.Contains([]int{16, 24, 32}, len(secrets.Cookie)) {
		return DexAuthenticatorSecrets{}, false, nil
	}
	return secrets, true, nil
}

// Remove deletes what is left of the dex-authenticator of the object key: the Dex client it had
// under its former ID, its Deployment with the objects around it and its Secret, and the registry
// Secret of the namespace when no other object of the namespace has a dex-authenticator of its
// own. It is called once the Service of the object points at the instance, or the object is gone.
func (l *DexAuthenticatorLegacy) Remove(ctx context.Context, key types.NamespacedName) error {
	pending, err := l.Has(ctx, key)
	if err != nil {
		return err
	}
	l.mu.Lock()
	_, former := l.former[key]
	l.mu.Unlock()
	if !pending && !former {
		return nil
	}

	// The client first, and whether anything else is left or not: it is found at the start by
	// itself, so an object that has only the client left still gets rid of it. Only a client taken
	// for this object at the start goes: one another object with the same former ID may have is
	// not this object's to remove.
	if former {
		if err := l.removeFormerClient(ctx, key); err != nil {
			return err
		}
		l.mu.Lock()
		delete(l.former, key)
		l.mu.Unlock()
	}
	if !pending {
		return nil
	}

	var errs []error
	name := legacyObjectName(key.Name)
	for _, gvk := range legacyKinds {
		errs = append(errs, l.remove(ctx, gvk, types.NamespacedName{Namespace: key.Namespace, Name: name}, key.Name))
	}
	errs = append(errs, l.remove(ctx, secretGVK, types.NamespacedName{Namespace: key.Namespace, Name: legacySecretName(key.Name)}, key.Name))
	if err := errors.Join(errs...); err != nil {
		return err
	}

	if l.done(key) {
		return l.remove(ctx, secretGVK, types.NamespacedName{Namespace: key.Namespace, Name: legacyRegistrySecret}, "")
	}
	return nil
}

// removeFormerClient deletes the Dex client the object key had under its former ID, if the chart
// rendered one and the module hook handed it over.
func (l *DexAuthenticatorLegacy) removeFormerClient(ctx context.Context, key types.NamespacedName) error {
	clientKey := FormerDexAuthenticatorClientKey(key.Name, key.Namespace)
	obj := controller.Object(controller.OAuth2ClientGVK)
	err := l.reader.Get(ctx, clientKey, obj)
	if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get OAuth2Client %s: %w", clientKey, err)
	}
	if !IsFormerDexAuthenticatorClient(obj, key.Name, key.Namespace) {
		return nil
	}
	return l.deleteFormerClient(ctx, obj)
}

// deleteFormerClient deletes the Dex client obj, which is the client of a former ID.
func (l *DexAuthenticatorLegacy) deleteFormerClient(ctx context.Context, obj *unstructured.Unstructured) error {
	clientKey := client.ObjectKeyFromObject(obj)
	id, _, _ := unstructured.NestedString(obj.Object, "id")

	// The UID keeps an object created anew under the name in the meantime.
	var opts []client.DeleteOption
	if uid := obj.GetUID(); uid != "" {
		opts = append(opts, client.Preconditions{UID: &uid})
	}
	err := l.client.Delete(ctx, obj, opts...)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete OAuth2Client %s: %w", clientKey, err)
	}
	l.log.Info("deleted", "kind", controller.OAuth2ClientGVK.Kind, "namespace", clientKey.Namespace, "name", clientKey.Name,
		"clientID", id,
		"reason", "the pods that signed users in with the former ID of the DexAuthenticator serve it no longer")
	return nil
}

// remove deletes the object of the kind at key if it is an object of the dex-authenticator of the
// DexAuthenticator name.
func (l *DexAuthenticatorLegacy) remove(ctx context.Context, gvk schema.GroupVersionKind, key types.NamespacedName, name string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	err := l.reader.Get(ctx, key, obj)
	if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get %s %s: %w", gvk.Kind, key, err)
	}
	if !isLegacy(obj, name) {
		return nil
	}

	// The UID keeps an object created anew under the name in the meantime.
	var opts []client.DeleteOption
	if uid := obj.GetUID(); uid != "" {
		opts = append(opts, client.Preconditions{UID: &uid})
	}
	err = l.client.Delete(ctx, obj, opts...)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete %s %s: %w", gvk.Kind, key, err)
	}
	l.log.Info("deleted", "kind", gvk.Kind, "namespace", key.Namespace, "name", key.Name, "reason", "the shared dex-authenticator serves the DexAuthenticator")
	return nil
}

// formerClientOwner returns the object a Dex client of a former ID belongs to, of the objects
// keys whose former ID it has: the only one, or the only one that still has a dex-authenticator of
// its own.
func formerClientOwner(keys []types.NamespacedName, pending map[types.NamespacedName]struct{}) (types.NamespacedName, bool) {
	if len(keys) == 1 {
		return keys[0], true
	}
	var owner types.NamespacedName
	found := 0
	for _, key := range keys {
		if _, ok := pending[key]; ok {
			owner = key
			found++
		}
	}
	return owner, found == 1
}
