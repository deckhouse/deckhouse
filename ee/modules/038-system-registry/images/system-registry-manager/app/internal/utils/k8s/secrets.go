package k8s

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"golang.org/x/crypto/bcrypt"
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
func DeleteAllRegistryNodeSecrets(ctx context.Context, kubeClient *kubernetes.Clientset) ([]string, error) {
	secrets, err := GetAllRegistryNodeSecrets(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	var deletedSecrets []string
	var errs []error

	// Delete all secrets
	for _, secret := range secrets {
		err := kubeClient.CoreV1().Secrets(RegistryNamespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
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

// GetAllRegistryNodeSecrets returns all registry node secrets in the cluster
func GetAllRegistryNodeSecrets(ctx context.Context, kubeClient *kubernetes.Clientset) ([]corev1.Secret, error) {
	labelSelector := fmt.Sprintf("%s=%s,%s=%s", labelModuleKey, labelModuleValue, labelTypeKey, labelNodeSecretTypeValue)

	secrets, err := kubeClient.CoreV1().Secrets(RegistryNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
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

// GetRegistryUser returns the registry user by name
func GetRegistryUser(ctx context.Context, kubeClient *kubernetes.Clientset, userName string) (*RegistryUser, error) {
	// Get the secret by name
	secret, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Get(ctx, userName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &RegistryUser{
		UserName:       string(secret.Data["name"]),
		Password:       string(secret.Data["password"]),
		HashedPassword: string(secret.Data["passwordHash"]),
	}, nil
}

// CreateRegistryUser creates a new registry user in the cluster
func CreateRegistryUser(ctx context.Context, kubeClient *kubernetes.Clientset, userName string) (*RegistryUser, error) {
	// Get the secret by name
	_, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Get(ctx, userName, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("secret %s already exists", userName)
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// Generate a new password
	password, hashedPassword, err := generateRegistryPassword()
	if err != nil {
		return nil, err
	}

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
	createdSecret, err := kubeClient.CoreV1().Secrets(RegistryNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return &RegistryUser{
		UserName:       string(createdSecret.Data["name"]),
		Password:       string(createdSecret.Data["password"]),
		HashedPassword: string(createdSecret.Data["passwordHash"]),
	}, nil
}
