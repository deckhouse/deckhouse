/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package k8s_legacy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeSecretData struct {
	AuthCrt         []byte
	AuthKey         []byte
	DistributionCrt []byte
	DistributionKey []byte
}

type RegistryUser struct {
	UserName       string
	Password       string
	HashedPassword string
}

type CASecretData struct {
	CACertPEM        []byte
	CAKeyPEM         []byte
	AuthTokenCertPEM []byte
	AuthTokenKeyPEM  []byte
}

const (
	labelModuleKey           = "module"
	labelModuleValue         = "embedded-registry"
	labelTypeKey             = "type"
	labelNodeSecretTypeValue = "node-secret"
	labelHeritageKey         = "heritage"
	labelHeritageValue       = "deckhouse"

	labelCaSecretTypeValue = "ca-secret"

	registryUserPasswordLength  = 16
	registryUserPasswordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// DeleteAllRegistryNodeSecrets deletes all registry node secrets in the cluster
func DeleteAllRegistryNodeSecrets(ctx context.Context, kc client.Client) ([]string, error) {
	secrets, err := GetAllRegistryNodeSecrets(ctx, kc)
	if err != nil {
		return nil, err
	}

	var deletedSecrets []string
	var errs []error

	// Delete all secrets
	for _, secret := range secrets {
		err := kc.Delete(ctx, &secret)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete secret %s: %v", secret.Name, err))
			continue
		}
		deletedSecrets = append(deletedSecrets, secret.Name)
	}

	if len(errs) > 0 {
		return deletedSecrets, fmt.Errorf("errors occurred while deleting secrets: %v", errs)
	}

	return deletedSecrets, nil
}

func GetRegistryNodeSecret(ctx context.Context, kc client.Client, nodeName string) (*corev1.Secret, error) {
	secretName := fmt.Sprintf("registry-node-%s-pki", nodeName)

	secret := &corev1.Secret{}
	err := kc.Get(ctx, types.NamespacedName{Name: secretName, Namespace: RegistryNamespace}, secret)
	return secret, err
}

// GetAllRegistryNodeSecrets returns all registry node secrets in the cluster
func GetAllRegistryNodeSecrets(ctx context.Context, kc client.Client) ([]corev1.Secret, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		labelModuleKey: labelModuleValue,
		labelTypeKey:   labelNodeSecretTypeValue,
	})

	var secretList corev1.SecretList

	err := kc.List(ctx, &secretList, &client.ListOptions{
		Namespace:     RegistryNamespace,
		LabelSelector: labelSelector,
	})

	if err != nil {
		return nil, err
	}

	return secretList.Items, nil
}

// generateRegistryPassword generates a random password for the registry user
func generateRegistryPassword() (string, string, error) {
	password := make([]byte, registryUserPasswordLength)
	charsetLength := big.NewInt(int64(len(registryUserPasswordCharset)))

	for i := range password {
		index, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", "", err
		}
		password[i] = registryUserPasswordCharset[index.Int64()]
	}

	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}

	return string(password), string(hash), nil
}

// CreateRegistryUserSecret creates a new secret in the cluster with the given user data
func CreateRegistryUserSecret(ctx context.Context, kc client.Client, userName, password, hashedPassword string) (*RegistryUser, error) {
	// Create secret data and object
	secretData := map[string][]byte{
		"name":         []byte(userName),
		"password":     []byte(password),
		"passwordHash": []byte(hashedPassword),
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: RegistryNamespace,
			Labels: map[string]string{
				labelModuleKey:   labelModuleValue,
				labelHeritageKey: labelHeritageValue,
			},
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}

	// Create the secret in the cluster
	err := kc.Create(ctx, secret)
	if err != nil {
		return nil, err
	}

	return &RegistryUser{
		UserName:       string(secret.Data["name"]),
		Password:       string(secret.Data["password"]),
		HashedPassword: string(secret.Data["passwordHash"]),
	}, nil
}

// CreateNodePKISecret creates a new PKI secret for the provided node.
func CreateNodePKISecret(ctx context.Context, kc client.Client, node MasterNode, caCertPEM []byte, caKeyPEM []byte) ([]byte, []byte, []byte, []byte, error) {

	labelSelector := client.MatchingLabels(map[string]string{
		labelHeritageKey: labelHeritageValue,
		labelModuleKey:   labelModuleValue,
		labelTypeKey:     labelNodeSecretTypeValue,
	})

	nodeSecretName := fmt.Sprintf("registry-node-%s-pki", node.Name)

	hosts := []string{
		"127.0.0.1",
		"localhost",
		node.Address,
		fmt.Sprintf("%s.%s.svc", RegistrySvcName, RegistryNamespace),
	}

	// generate registry node distribution certificates
	distributionCert, distributionKey, err := generateCertificate("embedded-registry-distribution", hosts, caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// generate registry node auth certificates
	authCert, authKey, err := generateCertificate("embedded-registry-auth", hosts, caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// create secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeSecretName,
			Namespace: RegistryNamespace,
			Labels:    labelSelector,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			AuthCert:         authCert,
			AuthKey:          authKey,
			DistributionCert: distributionCert,
			DistributionKey:  distributionKey,
		},
	}

	// create secret in k8s
	err = kc.Create(ctx, secret)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return distributionCert, distributionKey, authCert, authKey, nil
}

// EnsureCASecret ensures that the CA secret exists in the cluster. If it does not exist, it generates a new CA certificate and key.
func EnsureCASecret(ctx context.Context, kc client.Client) (bool, CASecretData, error) {
	var caSecretStruct CASecretData

	// Check if the CA secret already exists
	registryCASecret, err := GetCASecret(ctx, kc)
	if err == nil {
		caSecretStruct.CACertPEM = registryCASecret.Data[RegistryCACert]
		caSecretStruct.CAKeyPEM = registryCASecret.Data[RegistryCAKey]
		caSecretStruct.AuthTokenCertPEM = registryCASecret.Data[AuthTokenCert]
		caSecretStruct.AuthTokenKeyPEM = registryCASecret.Data[AuthTokenKey]

		// Return the existing secret
		return false, caSecretStruct, nil
	}

	// if any error other than NotFound, return the error
	if !apierrors.IsNotFound(err) {
		return false, caSecretStruct, err
	}
	// Secret does not exist, generate a new CA certificate and key
	caSecretStruct.CACertPEM, caSecretStruct.CAKeyPEM, err = generateCA()
	if err != nil {
		return false, caSecretStruct, err
	}

	// Generate the auth token certificate and key
	caSecretStruct.AuthTokenCertPEM, caSecretStruct.AuthTokenKeyPEM, err = generateCertificate(
		"embedded-registry-auth-token",
		nil,
		caSecretStruct.CACertPEM,
		caSecretStruct.CAKeyPEM,
	)
	if err != nil {
		return false, caSecretStruct, err
	}

	// Create the CA secret
	if err := CreateRegistryCaPKISecret(ctx, kc, caSecretStruct); err != nil {
		return false, caSecretStruct, err
	}

	return true, caSecretStruct, nil
}

func GetCASecret(ctx context.Context, kc client.Client) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := kc.Get(ctx, types.NamespacedName{Name: "registry-pki", Namespace: RegistryNamespace}, secret)
	return secret, err
}

// CreateRegistryCaPKISecret creates a new CA secret in the specified namespace
func CreateRegistryCaPKISecret(ctx context.Context, kc client.Client, caSecretStruct CASecretData) error {
	secretLabels := map[string]string{
		labelHeritageKey: labelHeritageValue,
		labelModuleKey:   labelModuleValue,
		labelTypeKey:     labelCaSecretTypeValue,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-pki",
			Namespace: RegistryNamespace,
			Labels:    secretLabels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			RegistryCACert: caSecretStruct.CACertPEM,
			RegistryCAKey:  caSecretStruct.CAKeyPEM,
			AuthTokenCert:  caSecretStruct.AuthTokenCertPEM,
			AuthTokenKey:   caSecretStruct.AuthTokenKeyPEM,
		},
	}

	// Create the secret in Kubernetes
	return kc.Create(ctx, secret)
}
