/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"context"
	"fmt"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
)

func (r *RegistryReconciler) SecretsStartupCheckCreate(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Embedded registry startup initialization", "component", "registry-controller")

	// Lock mutex to ensure thread safety
	r.embeddedRegistry.mutex.Lock()
	defer r.embeddedRegistry.mutex.Unlock()

	// Get the required environment variables
	registryAddress := os.Getenv("REGISTRY_ADDRESS")
	registryPath := os.Getenv("REGISTRY_PATH")
	imageDockerAuth := os.Getenv("IMAGE_DOCKER_AUTH")
	imageDockerDistribution := os.Getenv("IMAGE_DOCKER_DISTRIBUTION")

	if registryAddress == "" || imageDockerAuth == "" || imageDockerDistribution == "" || registryPath == "" {
		return fmt.Errorf("missing required environment variables: REGISTRY_ADDRESS, REGISTRY_PATH, IMAGE_DOCKER_AUTH, or IMAGE_DOCKER_DISTRIBUTION")
	}

	// Fill the embedded registry images struct with the registry address and image names
	r.embeddedRegistry.images.DockerAuth = fmt.Sprintf("%s%s@%s", registryAddress, registryPath, imageDockerAuth)
	r.embeddedRegistry.images.DockerDistribution = fmt.Sprintf("%s%s@%s", registryAddress, registryPath, imageDockerDistribution)

	// Ensure CA certificate exists and create if not
	isGenerated, caCertStruct, err := k8s.EnsureCASecret(ctx, r.Client)
	if err != nil {
		return err
	}

	// If CA certificate was generated, delete all PKI secrets
	if isGenerated {
		logger.Info("New registry root CA generated", "secret", "registry-pki", "component", "registry-controller")

		// Delete all PKI secrets
		deletedSecrets, err := k8s.DeleteAllRegistryNodeSecrets(ctx, r.Client)
		if err != nil {
			return err
		}
		for _, deletedSecret := range deletedSecrets {
			logger.Info("Deleted node PKI secret, because CA certificate was regenerated", "secret", deletedSecret, "component", "registry-controller")
		}
	}

	// Fill the embedded registry struct with the CA PKI
	r.embeddedRegistry.caPKI = k8s.Certificate{
		Cert: caCertStruct.CACertPEM,
		Key:  caCertStruct.CAKeyPEM,
	}

	// Fill the embedded registry struct with the Auth Token PKI
	r.embeddedRegistry.authTokenPKI = k8s.Certificate{
		Cert: caCertStruct.AuthTokenCertPEM,
		Key:  caCertStruct.AuthTokenKeyPEM,
	}

	for masterNodeName, masterNode := range r.embeddedRegistry.masterNodes {
		// Check if the node PKI secret exists
		secret, err := k8s.GetRegistryNodeSecret(ctx, r.Client, masterNodeName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Create the node PKI secret if it doesn't exist
		if len(secret.Data) == 0 {
			dc, dk, ac, ak, err := k8s.CreateNodePKISecret(
				ctx,
				r.Client,
				masterNode,
				caCertStruct.CACertPEM,
				caCertStruct.CAKeyPEM,
			)
			if err != nil {
				return err
			}
			logger.Info("Node secret created", "nodeName", masterNodeName, "component", "registry-controller")

			masterNode.AuthCertificate = k8s.Certificate{
				Cert: ac,
				Key:  ak,
			}
			masterNode.DistributionCertificate = k8s.Certificate{
				Cert: dc,
				Key:  dk,
			}
		} else {
			// Extract the existing secret data
			masterNode.AuthCertificate = k8s.Certificate{
				Cert: secret.Data[k8s.AuthCert],
				Key:  secret.Data[k8s.AuthKey],
			}
			masterNode.DistributionCertificate = k8s.Certificate{
				Cert: secret.Data[k8s.DistributionCert],
				Key:  secret.Data[k8s.DistributionKey],
			}
		}

		// Add the node to the embedded registry struct
		r.embeddedRegistry.masterNodes[masterNode.Name] = masterNode
	}

	// Ensure registry user secrets exist and create if not
	var registryUserRwSecret *k8s.RegistryUser
	var registryUserRoSecret *k8s.RegistryUser

	registryUserRwSecret, err = k8s.GetRegistryUser(ctx, r.Client, "registry-user-rw")
	if err != nil {
		if apierrors.IsNotFound(err) {
			if registryUserRwSecret, err = k8s.CreateRegistryUser(ctx, r.Client, "registry-user-rw"); err != nil {
				return fmt.Errorf("cannot create registry rw user secret: %w", err)
			}

			logger.Info("Created registry rw user secret", "component", "registry-controller")
		} else {
			return fmt.Errorf("cannot get regstry rw user: %w", err)
		}
	}

	registryUserRoSecret, err = k8s.GetRegistryUser(ctx, r.Client, "registry-user-ro")
	if err != nil {
		if apierrors.IsNotFound(err) {
			if registryUserRoSecret, err = k8s.CreateRegistryUser(ctx, r.Client, "registry-user-ro"); err != nil {
				return fmt.Errorf("cannot create registry ro user secret: %w", err)
			}

			logger.Info("Created registry ro user secret", "component", "registry-controller")
		} else {
			return fmt.Errorf("cannot get regstry ro user: %w", err)
		}
	}

	// Fill the embedded registry struct with the registry user secrets
	r.embeddedRegistry.registryRwUser = *registryUserRwSecret
	r.embeddedRegistry.registryRoUser = *registryUserRoSecret

	logger.Info("Embedded registry startup initialization complete", "component", "registry-controller")
	return nil

}
