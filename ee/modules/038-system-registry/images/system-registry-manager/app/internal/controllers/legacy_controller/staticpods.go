/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"fmt"
	"net/http"

	staticpod "embeded-registry-manager/internal/static-pod"
	k8s "embeded-registry-manager/internal/utils/k8s_legacy"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *RegistryReconciler) syncRegistryStaticPods(ctx context.Context, node k8s.MasterNode) ([]byte, error) {

	// Prepare the upstream registry struct
	var upstreamRegistry staticpod.UpstreamRegistry
	if r.embeddedRegistry.mc.Settings.Mode == "Proxy" {
		upstreamRegistry = r.prepareUpstreamRegistry()
	}

	// Prepare the embedded registry config struct
	data := r.prepareEmbeddedRegistryConfig(node, upstreamRegistry)

	return r.createNodeRegistry(ctx, node.Name, data)
}

func (r *RegistryReconciler) deleteNodeRegistry(ctx context.Context, nodeName string) ([]byte, error) {
	// Get the pod IP for the node
	podIP, err := r.getPodIPForNode(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	return r.HttpClient.Send(fmt.Sprintf("https://%s:4577/staticpod/delete", podIP), http.MethodDelete, nil)
}

func (r *RegistryReconciler) createNodeRegistry(ctx context.Context, nodeName string, data staticpod.EmbeddedRegistryConfig) ([]byte, error) {
	// Get the pod IP for the node
	podIP, err := r.getPodIPForNode(ctx, nodeName)
	if err != nil {
		return nil, err
	}
	return r.HttpClient.Send(fmt.Sprintf("https://%s:4577/staticpod/create", podIP), http.MethodPost, data)
}

func (r *RegistryReconciler) prepareUpstreamRegistry() staticpod.UpstreamRegistry {
	return staticpod.UpstreamRegistry{
		Scheme:   r.embeddedRegistry.mc.Settings.Proxy.Scheme,
		Host:     r.embeddedRegistry.mc.Settings.Proxy.Host,
		Path:     r.embeddedRegistry.mc.Settings.Proxy.Path,
		CA:       r.embeddedRegistry.mc.Settings.Proxy.CA,
		User:     r.embeddedRegistry.mc.Settings.Proxy.User,
		Password: r.embeddedRegistry.mc.Settings.Proxy.Password,
		TTL:      r.embeddedRegistry.mc.Settings.Proxy.TTL.StringPointer(),
	}
}

func (r *RegistryReconciler) prepareEmbeddedRegistryConfig(node k8s.MasterNode, upstreamRegistry staticpod.UpstreamRegistry) staticpod.EmbeddedRegistryConfig {
	return staticpod.EmbeddedRegistryConfig{
		Registry: staticpod.RegistryDetails{
			UserRw: staticpod.User{
				Name:         r.embeddedRegistry.registryRwUser.UserName,
				PasswordHash: r.embeddedRegistry.registryRwUser.HashedPassword,
			},
			UserRo: staticpod.User{
				Name:         r.embeddedRegistry.registryRoUser.UserName,
				PasswordHash: r.embeddedRegistry.registryRoUser.HashedPassword,
			},
			RegistryMode:     r.embeddedRegistry.mc.Settings.Mode,
			HttpSecret:       "http-secret",
			UpstreamRegistry: upstreamRegistry, // Will be empty for non-Proxy modes
		},
		Images: staticpod.Images{
			DockerDistribution: r.embeddedRegistry.images.DockerDistribution,
			DockerAuth:         r.embeddedRegistry.images.DockerAuth,
		},
		Pki: staticpod.Pki{
			CaCert:           string(r.embeddedRegistry.caPKI.Cert),
			AuthCert:         string(node.AuthCertificate.Cert),
			AuthKey:          string(node.AuthCertificate.Key),
			AuthTokenCert:    string(r.embeddedRegistry.authTokenPKI.Cert),
			AuthTokenKey:     string(r.embeddedRegistry.authTokenPKI.Key),
			DistributionCert: string(node.DistributionCertificate.Cert),
			DistributionKey:  string(node.DistributionCertificate.Key),
		},
	}
}

func (r *RegistryReconciler) getPodIPForNode(ctx context.Context, nodeName string) (string, error) {
	var pods corev1.PodList
	err := r.listWithFallback(ctx, &pods, client.MatchingLabels{
		"app": "system-registry-staticpod-manager",
	}, client.MatchingFields{
		"spec.nodeName": nodeName,
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("system-registry-staticpod-manager pod not found for node %s", nodeName)
	}
	if pods.Items[0].Status.PodIP == "" {
		return "", fmt.Errorf("system-registry-staticpod-manager pod IP is empty for node %s", nodeName)
	}

	return pods.Items[0].Status.PodIP, nil
}
