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

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = "metal3instance.deckhouse.io/finalizer"

	credentialsSecretType  = "cloud-provider.deckhouse.io/credentials"
	authSchemeUserPassword = "userPassword"

	annotationInstance          = "metal3.deckhouse.io/instance"
	annotationInstanceNamespace = "metal3.deckhouse.io/instance-namespace"
	managedLabelsAnnotation     = "metal3.deckhouse.io/managed-label-keys"

	deleteRequeueAfter = 15 * time.Second
)

var (
	metal3InstanceGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "Metal3Instance"}
	bareMetalHostGVK  = schema.GroupVersionKind{Group: "metal3.io", Version: "v1alpha1", Kind: "BareMetalHost"}
)

type reconciler struct {
	client.Client
	targetNamespace string
	resolver        BMCResolver
}

type instanceSpec struct {
	Online         bool
	BootMACAddress string
	BMC            BMCConfig
	CredentialsRef objectReference
}

type objectReference struct {
	Kind string
	Name string
}

type credentials struct {
	Username        string
	Password        string
	ResourceVersion string
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(metal3InstanceGVK)
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, instance)
	}
	if !controllerutil.ContainsFinalizer(instance, finalizerName) {
		controllerutil.AddFinalizer(instance, finalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if instance.GetNamespace() != r.targetNamespace {
		return r.fail(ctx, instance, "InvalidNamespace", fmt.Errorf("Metal3Instance must be created in namespace %q", r.targetNamespace))
	}
	spec, err := readSpec(instance)
	if err != nil {
		return r.fail(ctx, instance, "InvalidSpec", err)
	}
	creds, err := r.readCredentials(ctx, instance, spec.CredentialsRef)
	if err != nil {
		return r.fail(ctx, instance, "CredentialsInvalid", err)
	}

	resolved, ok := cachedResolvedBMC(instance, spec, creds.ResourceVersion)
	if !ok {
		resolved, err = r.resolver.Resolve(ctx, spec.BMC, creds.Username, creds.Password)
		if err != nil {
			return r.fail(ctx, instance, "BMCResolutionFailed", err)
		}
	}

	secretName := r.generatedSecretName(instance)
	if err := r.ensureCredentialSecret(ctx, instance, creds, secretName); err != nil {
		return ctrl.Result{}, err
	}
	bmh, err := r.ensureBareMetalHost(ctx, instance, spec, resolved, secretName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.setStatus(ctx, instance, bmh, secretName, resolved, creds.ResourceVersion, "", true, "BMCResolved"); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func readSpec(instance *unstructured.Unstructured) (instanceSpec, error) {
	spec := instanceSpec{}
	spec.Online, _, _ = unstructured.NestedBool(instance.Object, "spec", "online")
	mac, _, _ := unstructured.NestedString(instance.Object, "spec", "bootMACAddress")
	spec.BootMACAddress = strings.ToLower(mac)
	spec.BMC.IPAddress, _, _ = unstructured.NestedString(instance.Object, "spec", "bmc", "ipAddress")
	port, ok, _ := unstructured.NestedInt64(instance.Object, "spec", "bmc", "port")
	if ok {
		spec.BMC.Port = int(port)
	}
	spec.BMC.SystemUUID, _, _ = unstructured.NestedString(instance.Object, "spec", "bmc", "systemUUID")
	spec.BMC.Insecure, _, _ = unstructured.NestedBool(instance.Object, "spec", "bmc", "insecure")
	spec.CredentialsRef.Kind, _, _ = unstructured.NestedString(instance.Object, "spec", "bmc", "credentialsRef", "kind")
	spec.CredentialsRef.Name, _, _ = unstructured.NestedString(instance.Object, "spec", "bmc", "credentialsRef", "name")

	if spec.BootMACAddress == "" {
		return spec, fmt.Errorf("spec.bootMACAddress is required")
	}
	if spec.BMC.IPAddress == "" {
		return spec, fmt.Errorf("spec.bmc.ipAddress is required")
	}
	if spec.BMC.SystemUUID == "" {
		return spec, fmt.Errorf("spec.bmc.systemUUID is required")
	}
	if spec.CredentialsRef.Kind != "Secret" || spec.CredentialsRef.Name == "" {
		return spec, fmt.Errorf("spec.bmc.credentialsRef must reference a named Secret")
	}
	return spec, nil
}

func (r *reconciler) readCredentials(ctx context.Context, instance *unstructured.Unstructured, ref objectReference) (credentials, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: instance.GetNamespace(), Name: ref.Name}
	if err := r.Get(ctx, key, secret); err != nil {
		return credentials{}, fmt.Errorf("get BMC credentials Secret %s: %w", key, err)
	}
	if string(secret.Type) != credentialsSecretType {
		return credentials{}, fmt.Errorf("Secret %s must have type %q", key, credentialsSecretType)
	}
	if string(secret.Data["authScheme"]) != authSchemeUserPassword {
		return credentials{}, fmt.Errorf("Secret %s must use authScheme %q", key, authSchemeUserPassword)
	}
	username := string(secret.Data["identity"])
	password := string(secret.Data["secret"])
	if username == "" || password == "" {
		return credentials{}, fmt.Errorf("Secret %s must contain non-empty identity and secret keys", key)
	}
	return credentials{Username: username, Password: password, ResourceVersion: secret.ResourceVersion}, nil
}

func (r *reconciler) ensureCredentialSecret(ctx context.Context, instance *unstructured.Unstructured, creds credentials, name string) error {
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: name}
	target := &corev1.Secret{}
	err := r.Get(ctx, key, target)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	desiredData := map[string][]byte{"username": []byte(creds.Username), "password": []byte(creds.Password)}
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name, Annotations: instanceOwnershipAnnotations(instance)},
			Type:       corev1.SecretTypeOpaque,
			Data:       desiredData,
		})
	}
	updated := false
	if !reflect.DeepEqual(target.Data, desiredData) {
		target.Data = desiredData
		updated = true
	}
	if target.Type != corev1.SecretTypeOpaque {
		target.Type = corev1.SecretTypeOpaque
		updated = true
	}
	if ensureOwnershipAnnotations(target, instance) {
		updated = true
	}
	if !updated {
		return nil
	}
	return r.Update(ctx, target)
}

func (r *reconciler) ensureBareMetalHost(ctx context.Context, instance *unstructured.Unstructured, spec instanceSpec, resolved ResolvedBMC, secretName string) (*unstructured.Unstructured, error) {
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: instance.GetName()}
	err := r.Get(ctx, key, bmh)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		bmh.SetNamespace(key.Namespace)
		bmh.SetName(key.Name)
		bmh.SetLabels(desiredBareMetalHostLabels(instance))
		bmh.SetAnnotations(instanceOwnershipAnnotations(instance))
		setManagedLabelKeys(bmh, instance.GetLabels())
		if err := unstructured.SetNestedMap(bmh.Object, desiredBareMetalHostSpec(spec, resolved, secretName), "spec"); err != nil {
			return nil, err
		}
		return bmh, r.Create(ctx, bmh)
	}

	updated, err := updateBareMetalHostSpec(bmh, spec, resolved, secretName)
	if err != nil {
		return nil, err
	}
	if syncBareMetalHostLabels(bmh, instance) {
		updated = true
	}
	if !updated {
		return bmh, nil
	}
	return bmh, r.Update(ctx, bmh)
}

func desiredBareMetalHostSpec(spec instanceSpec, resolved ResolvedBMC, secretName string) map[string]interface{} {
	bmc := map[string]interface{}{"address": resolved.Address, "credentialsName": secretName}
	if spec.BMC.Insecure {
		bmc["disableCertificateVerification"] = true
	}
	return map[string]interface{}{
		"online":         spec.Online,
		"bootMACAddress": spec.BootMACAddress,
		"bmc":            bmc,
	}
}

func updateBareMetalHostSpec(bmh *unstructured.Unstructured, spec instanceSpec, resolved ResolvedBMC, secretName string) (bool, error) {
	updated := false
	for _, field := range []struct {
		value string
		path  []string
	}{
		{spec.BootMACAddress, []string{"spec", "bootMACAddress"}},
		{resolved.Address, []string{"spec", "bmc", "address"}},
		{secretName, []string{"spec", "bmc", "credentialsName"}},
	} {
		changed, err := setNestedString(bmh, field.value, field.path...)
		if err != nil {
			return false, err
		}
		updated = updated || changed
	}
	for _, field := range []struct {
		value bool
		path  []string
	}{
		{spec.Online, []string{"spec", "online"}},
		{spec.BMC.Insecure, []string{"spec", "bmc", "disableCertificateVerification"}},
	} {
		changed, err := setNestedBool(bmh, field.value, field.path...)
		if err != nil {
			return false, err
		}
		updated = updated || changed
	}
	return updated, nil
}

func setNestedString(obj *unstructured.Unstructured, value string, fields ...string) (bool, error) {
	current, ok, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil || ok && current == value {
		return false, err
	}
	return true, unstructured.SetNestedField(obj.Object, value, fields...)
}

func setNestedBool(obj *unstructured.Unstructured, value bool, fields ...string) (bool, error) {
	current, ok, err := unstructured.NestedBool(obj.Object, fields...)
	if err != nil || ok && current == value {
		return false, err
	}
	return true, unstructured.SetNestedField(obj.Object, value, fields...)
}

func cachedResolvedBMC(instance *unstructured.Unstructured, spec instanceSpec, credentialsVersion string) (ResolvedBMC, bool) {
	if observed, _, _ := unstructured.NestedInt64(instance.Object, "status", "observedGeneration"); observed != instance.GetGeneration() {
		return ResolvedBMC{}, false
	}
	version, _, _ := unstructured.NestedString(instance.Object, "status", "bmc", "credentialsVersion")
	address, _, _ := unstructured.NestedString(instance.Object, "status", "bmc", "address")
	protocol, _, _ := unstructured.NestedString(instance.Object, "status", "bmc", "protocol")
	uuid, _, _ := unstructured.NestedString(instance.Object, "status", "bmc", "systemUUID")
	if version != credentialsVersion || address == "" || uuid == "" || !equalUUID(uuid, spec.BMC.SystemUUID) {
		return ResolvedBMC{}, false
	}
	return ResolvedBMC{Address: address, Protocol: protocol, SystemUUID: uuid}, true
}

func (r *reconciler) fail(ctx context.Context, instance *unstructured.Unstructured, reason string, cause error) (ctrl.Result, error) {
	if err := r.setStatus(ctx, instance, nil, "", ResolvedBMC{}, "", cause.Error(), false, reason); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, cause
}

func (r *reconciler) setStatus(ctx context.Context, instance, bmh *unstructured.Unstructured, secretName string, resolved ResolvedBMC, credentialsVersion, message string, resolvedOK bool, reason string) error {
	status := map[string]interface{}{
		"observedGeneration": instance.GetGeneration(),
		"conditions":         []interface{}{conditionMap(instance, resolvedOK, reason, message)},
	}
	if bmh != nil {
		status["bareMetalHost"] = r.bareMetalHostStatus(bmh)
	}
	if secretName != "" {
		status["credentialsSecret"] = map[string]interface{}{"name": secretName, "namespace": r.targetNamespace}
	}
	if resolved.Address != "" {
		status["bmc"] = map[string]interface{}{
			"protocol":           resolved.Protocol,
			"address":            resolved.Address,
			"systemUUID":         resolved.SystemUUID,
			"credentialsVersion": credentialsVersion,
		}
	}
	if message != "" {
		status["message"] = message
	}
	if reflect.DeepEqual(instance.Object["status"], status) {
		return nil
	}
	base := instance.DeepCopy()
	instance.Object["status"] = status
	return r.Status().Patch(ctx, instance, client.MergeFrom(base))
}

func conditionMap(instance *unstructured.Unstructured, ok bool, reason, message string) map[string]interface{} {
	status := "False"
	if ok {
		status = "True"
	}
	transition := metav1.Now()
	oldConditions, _, _ := unstructured.NestedSlice(instance.Object, "status", "conditions")
	for _, raw := range oldConditions {
		condition, isMap := raw.(map[string]interface{})
		if !isMap || condition["type"] != "BMCResolved" || condition["status"] != status || condition["reason"] != reason {
			continue
		}
		if old, valid := condition["lastTransitionTime"].(string); valid {
			if parsed, err := time.Parse(time.RFC3339, old); err == nil {
				transition = metav1.NewTime(parsed)
			}
		}
	}
	return map[string]interface{}{
		"type":               "BMCResolved",
		"status":             status,
		"reason":             reason,
		"message":            message,
		"observedGeneration": instance.GetGeneration(),
		"lastTransitionTime": transition.Format(time.RFC3339),
	}
}

func (r *reconciler) reconcileDelete(ctx context.Context, instance *unstructured.Unstructured) (ctrl.Result, error) {
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: instance.GetName()}
	if err := r.Get(ctx, key, bmh); err == nil {
		if bmh.GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, bmh); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: deleteRequeueAfter}, nil
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: r.targetNamespace, Name: r.generatedSecretName(instance)}}
	if err := r.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	controllerutil.RemoveFinalizer(instance, finalizerName)
	return ctrl.Result{}, r.Update(ctx, instance)
}

func (r *reconciler) bareMetalHostToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	annotations := obj.GetAnnotations()
	if annotations[annotationInstance] == "" || annotations[annotationInstanceNamespace] == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: annotations[annotationInstanceNamespace], Name: annotations[annotationInstance]}}}
}

func (r *reconciler) secretToInstances(ctx context.Context, obj client.Object) []reconcile.Request {
	instances := &unstructured.UnstructuredList{}
	instances.SetGroupVersionKind(metal3InstanceGVK.GroupVersion().WithKind("Metal3InstanceList"))
	if err := r.List(ctx, instances, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for i := range instances.Items {
		name, _, _ := unstructured.NestedString(instances.Items[i].Object, "spec", "bmc", "credentialsRef", "name")
		if name == obj.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: instances.Items[i].GetName()}})
		}
	}
	return requests
}

func (r *reconciler) bareMetalHostStatus(bmh *unstructured.Unstructured) map[string]interface{} {
	status := map[string]interface{}{"name": bmh.GetName(), "namespace": bmh.GetNamespace()}
	if state, ok, _ := unstructured.NestedString(bmh.Object, "status", "provisioning", "state"); ok {
		status["state"] = state
	}
	if online, ok, _ := unstructured.NestedBool(bmh.Object, "spec", "online"); ok {
		status["online"] = online
	}
	if consumer := bareMetalHostConsumer(bmh); consumer != "" {
		status["consumer"] = consumer
	}
	if message, ok, _ := unstructured.NestedString(bmh.Object, "status", "errorMessage"); ok && message != "" {
		status["error"] = message
	}
	return status
}

func bareMetalHostConsumer(bmh *unstructured.Unstructured) string {
	if name, ok, _ := unstructured.NestedString(bmh.Object, "status", "consumerRef", "name"); ok {
		return name
	}
	name, _, _ := unstructured.NestedString(bmh.Object, "spec", "consumerRef", "name")
	return name
}

func (r *reconciler) generatedSecretName(instance *unstructured.Unstructured) string {
	const suffix = "-bmh-credentials"
	name := instance.GetName()
	if len(name)+len(suffix) <= 253 {
		return name + suffix
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))[:8]
	prefixLength := 253 - len(suffix) - len(digest) - 1
	prefix := strings.TrimRight(name[:prefixLength], ".-")
	return prefix + "-" + digest + suffix
}

func instanceOwnershipAnnotations(instance *unstructured.Unstructured) map[string]string {
	return map[string]string{annotationInstance: instance.GetName(), annotationInstanceNamespace: instance.GetNamespace()}
}

func ensureOwnershipAnnotations(obj metav1.Object, instance *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	changed := false
	for key, value := range instanceOwnershipAnnotations(instance) {
		if annotations[key] != value {
			annotations[key] = value
			changed = true
		}
	}
	obj.SetAnnotations(annotations)
	return changed
}

func desiredBareMetalHostLabels(instance *unstructured.Unstructured) map[string]string {
	labels := make(map[string]string, len(instance.GetLabels()))
	for key, value := range instance.GetLabels() {
		labels[key] = value
	}
	return labels
}

func syncBareMetalHostLabels(bmh, instance *unstructured.Unstructured) bool {
	labels := bmh.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	before := copyStringMap(labels)
	beforeManagedKeys := bmh.GetAnnotations()[managedLabelsAnnotation]
	for _, key := range managedLabelKeys(bmh) {
		delete(labels, key)
	}
	for key, value := range instance.GetLabels() {
		labels[key] = value
	}
	bmh.SetLabels(labels)
	ownershipChanged := ensureOwnershipAnnotations(bmh, instance)
	setManagedLabelKeys(bmh, instance.GetLabels())
	return ownershipChanged || !reflect.DeepEqual(before, labels) || beforeManagedKeys != bmh.GetAnnotations()[managedLabelsAnnotation]
}

func managedLabelKeys(bmh *unstructured.Unstructured) []string {
	var keys []string
	_ = json.Unmarshal([]byte(bmh.GetAnnotations()[managedLabelsAnnotation]), &keys)
	return keys
}

func setManagedLabelKeys(bmh *unstructured.Unstructured, labels map[string]string) {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	raw, _ := json.Marshal(keys)
	annotations := bmh.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[managedLabelsAnnotation] = string(raw)
	bmh.SetAnnotations(annotations)
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
