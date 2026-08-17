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
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = "metal3instance.internal.deckhouse.io"

	labelInstance          = "metal3.deckhouse.io/instance"
	labelInstanceNamespace = "metal3.deckhouse.io/instance-namespace"
	labelPool              = "pool"

	deleteRequeueAfter = 15 * time.Second
)

var (
	metal3InstanceGVK = schema.GroupVersionKind{
		Group:   "internal.deckhouse.io",
		Version: "v1alpha1",
		Kind:    "Metal3Instance",
	}

	bareMetalHostGVK = schema.GroupVersionKind{
		Group:   "metal3.io",
		Version: "v1alpha1",
		Kind:    "BareMetalHost",
	}
)

type reconciler struct {
	client.Client

	targetNamespace              string
	defaultOnline                bool
	defaultAutomatedCleaningMode string
}

func main() {
	var targetNamespace string
	var defaultOnline bool
	var defaultAutomatedCleaningMode string

	flag.StringVar(&targetNamespace, "target-namespace", "d8-cloud-instance-manager", "namespace where generated BareMetalHost resources are created")
	flag.BoolVar(&defaultOnline, "default-online", true, "default BareMetalHost online value")
	flag.StringVar(&defaultAutomatedCleaningMode, "default-automated-cleaning-mode", "disabled", "default BareMetalHost automatedCleaningMode value")
	flag.Parse()

	zapLogger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger: %v\n", err)
		os.Exit(1)
	}
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "register core scheme: %v\n", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create manager: %v\n", err)
		os.Exit(1)
	}

	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(metal3InstanceGVK)
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)

	r := &reconciler{
		Client:                       mgr.GetClient(),
		targetNamespace:              targetNamespace,
		defaultOnline:                defaultOnline,
		defaultAutomatedCleaningMode: defaultAutomatedCleaningMode,
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(instance).
		Watches(bmh, handler.EnqueueRequestsFromMapFunc(r.bareMetalHostToInstance)).
		Complete(r); err != nil {
		fmt.Fprintf(os.Stderr, "create controller: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "run manager: %v\n", err)
		os.Exit(1)
	}
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

	spec, err := r.readSpec(instance)
	if err != nil {
		return ctrl.Result{}, r.setStatus(ctx, instance, nil, "", err.Error())
	}

	if err := r.ensureCredentialSecret(ctx, instance, spec); err != nil {
		return ctrl.Result{}, err
	}

	bmh, err := r.ensureBareMetalHost(ctx, instance, spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.setStatus(ctx, instance, bmh, r.generatedSecretName(instance), "")
}

func (r *reconciler) reconcileDelete(ctx context.Context, instance *unstructured.Unstructured) (ctrl.Result, error) {
	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: instance.GetName()}
	if err := r.Get(ctx, key, bmh); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else {
		if bmh.GetDeletionTimestamp().IsZero() {
			if err := r.Delete(ctx, bmh); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
		if err := r.setStatus(ctx, instance, bmh, r.generatedSecretName(instance), "waiting for BareMetalHost deletion"); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: deleteRequeueAfter}, nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.targetNamespace,
			Name:      r.generatedSecretName(instance),
		},
	}
	if err := r.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(instance, finalizerName)
	return ctrl.Result{}, r.Update(ctx, instance)
}

type instanceSpec struct {
	Pool                            string
	Online                          bool
	BootMACAddress                  string
	AutomatedCleaningMode           string
	BMCAddress                      string
	BMCCredentialsName              string
	BMCDisableCertificateValidation bool
}

func (r *reconciler) readSpec(instance *unstructured.Unstructured) (instanceSpec, error) {
	spec := instanceSpec{
		Online:                r.defaultOnline,
		AutomatedCleaningMode: r.defaultAutomatedCleaningMode,
	}

	if pool, ok, _ := unstructured.NestedString(instance.Object, "spec", "pool"); ok {
		spec.Pool = pool
	}
	if online, ok, _ := unstructured.NestedBool(instance.Object, "spec", "online"); ok {
		spec.Online = online
	}
	if mode, ok, _ := unstructured.NestedString(instance.Object, "spec", "automatedCleaningMode"); ok {
		spec.AutomatedCleaningMode = mode
	}
	if mac, ok, _ := unstructured.NestedString(instance.Object, "spec", "bootMACAddress"); ok {
		spec.BootMACAddress = strings.ToLower(mac)
	}
	if mac, ok, _ := unstructured.NestedString(instance.Object, "spec", "bmc", "bootMACAddress"); ok && spec.BootMACAddress == "" {
		spec.BootMACAddress = strings.ToLower(mac)
	}
	if online, ok, _ := unstructured.NestedBool(instance.Object, "spec", "bmc", "online"); ok {
		spec.Online = online
	}
	if address, ok, _ := unstructured.NestedString(instance.Object, "spec", "bmc", "address"); ok {
		spec.BMCAddress = address
	}
	if credentialsName, ok, _ := unstructured.NestedString(instance.Object, "spec", "bmc", "credentialsName"); ok {
		spec.BMCCredentialsName = credentialsName
	}
	if skipVerify, ok, _ := unstructured.NestedBool(instance.Object, "spec", "bmc", "disableCertificateVerification"); ok {
		spec.BMCDisableCertificateValidation = skipVerify
	}

	if spec.BMCAddress == "" {
		return spec, fmt.Errorf("spec.bmc.address is required")
	}
	if spec.BMCCredentialsName == "" {
		return spec, fmt.Errorf("spec.bmc.credentialsName is required")
	}
	if spec.BootMACAddress == "" && !strings.HasPrefix(spec.BMCAddress, "redfish-virtualmedia://") {
		return spec, fmt.Errorf("spec.bootMACAddress is required for non-virtual-media BMC addresses")
	}

	return spec, nil
}

func (r *reconciler) ensureCredentialSecret(ctx context.Context, instance *unstructured.Unstructured, spec instanceSpec) error {
	source := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: instance.GetNamespace(), Name: spec.BMCCredentialsName}, source); err != nil {
		return fmt.Errorf("get BMC credentials secret %s/%s: %w", instance.GetNamespace(), spec.BMCCredentialsName, err)
	}

	target := &corev1.Secret{}
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: r.generatedSecretName(instance)}
	if err := r.Get(ctx, key, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		target = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
				Labels: map[string]string{
					labelInstance: instance.GetName(),
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: copySecretData(source.Data),
		}
		return r.Create(ctx, target)
	}

	if target.Labels == nil {
		target.Labels = map[string]string{}
	}
	target.Labels[labelInstance] = instance.GetName()
	target.Type = corev1.SecretTypeOpaque
	target.Data = copySecretData(source.Data)
	return r.Update(ctx, target)
}

func (r *reconciler) ensureBareMetalHost(ctx context.Context, instance *unstructured.Unstructured, spec instanceSpec) (*unstructured.Unstructured, error) {
	desired := map[string]interface{}{
		"online":                spec.Online,
		"automatedCleaningMode": spec.AutomatedCleaningMode,
		"bmc": map[string]interface{}{
			"address":         spec.BMCAddress,
			"credentialsName": r.generatedSecretName(instance),
		},
	}
	if spec.BootMACAddress != "" {
		desired["bootMACAddress"] = spec.BootMACAddress
	}
	if spec.BMCDisableCertificateValidation {
		desired["bmc"].(map[string]interface{})["disableCertificateVerification"] = true
	}

	labels := map[string]string{
		labelInstance:          instance.GetName(),
		labelInstanceNamespace: instance.GetNamespace(),
	}
	if spec.Pool != "" {
		labels[labelPool] = spec.Pool
	}

	bmh := &unstructured.Unstructured{}
	bmh.SetGroupVersionKind(bareMetalHostGVK)
	key := types.NamespacedName{Namespace: r.targetNamespace, Name: instance.GetName()}
	if err := r.Get(ctx, key, bmh); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		bmh.SetNamespace(key.Namespace)
		bmh.SetName(key.Name)
		bmh.SetLabels(labels)
		if err := unstructured.SetNestedMap(bmh.Object, desired, "spec"); err != nil {
			return nil, err
		}
		return bmh, r.Create(ctx, bmh)
	}

	existingLabels := bmh.GetLabels()
	if existingLabels == nil {
		existingLabels = map[string]string{}
	}
	for key, value := range labels {
		existingLabels[key] = value
	}
	bmh.SetLabels(existingLabels)
	if err := unstructured.SetNestedMap(bmh.Object, desired, "spec"); err != nil {
		return nil, err
	}
	return bmh, r.Update(ctx, bmh)
}

func (r *reconciler) setStatus(ctx context.Context, instance *unstructured.Unstructured, bmh *unstructured.Unstructured, secretName, message string) error {
	status := map[string]interface{}{
		"observedGeneration": instance.GetGeneration(),
	}
	if bmh != nil {
		status["bareMetalHost"] = r.bareMetalHostStatus(bmh)
	}
	if secretName != "" {
		status["credentialsSecret"] = map[string]interface{}{
			"name":      secretName,
			"namespace": r.targetNamespace,
		}
	}
	if message != "" {
		status["message"] = message
	}
	instance.Object["status"] = status
	return r.Status().Update(ctx, instance)
}

func (r *reconciler) bareMetalHostToInstance(_ context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if labels == nil {
		return nil
	}

	name := labels[labelInstance]
	namespace := labels[labelInstanceNamespace]
	if name == "" || namespace == "" {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}}
}

func (r *reconciler) bareMetalHostStatus(bmh *unstructured.Unstructured) map[string]interface{} {
	status := map[string]interface{}{
		"name":      bmh.GetName(),
		"namespace": bmh.GetNamespace(),
	}

	if state, ok, _ := unstructured.NestedString(bmh.Object, "status", "provisioning", "state"); ok {
		status["state"] = state
	}
	if online, ok, _ := unstructured.NestedBool(bmh.Object, "spec", "online"); ok {
		status["online"] = online
	}
	if consumer := bareMetalHostConsumer(bmh); consumer != "" {
		status["consumer"] = consumer
	}
	if message, ok, _ := unstructured.NestedString(bmh.Object, "status", "errorMessage"); ok {
		status["error"] = message
	}

	return status
}

func bareMetalHostConsumer(bmh *unstructured.Unstructured) string {
	if name, ok, _ := unstructured.NestedString(bmh.Object, "status", "consumerRef", "name"); ok {
		return name
	}
	if name, ok, _ := unstructured.NestedString(bmh.Object, "spec", "consumerRef", "name"); ok {
		return name
	}
	return ""
}

func (r *reconciler) generatedSecretName(instance *unstructured.Unstructured) string {
	return instance.GetName() + "-bmc"
}

func copySecretData(in map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(in))
	for key, value := range in {
		out[key] = append([]byte(nil), value...)
	}
	return out
}
