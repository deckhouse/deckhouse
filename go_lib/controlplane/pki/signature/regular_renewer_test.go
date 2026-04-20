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

package signature

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func GenerateTmpSignatureCerts(t *testing.T, clientset kubernetes.Interface, certDurationDays int) (filePath string) {
	tempDir := t.TempDir()
	jwkFilePath := filepath.Join(tempDir, SignaturePrivateJWK)
	jwksFilePath := filepath.Join(tempDir, SignaturePublicJWKS)

	// Generate a test certificate that expires in 1 day
	certs, err := generateTLSCertificate("signature", "apiserver", CertKeyTypeED25519, certDurationDays)
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	privKey, ok := certs.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatalf("unexpected key type, expected ed25519.PrivateKey")
	}
	timeExpired := time.Now().AddDate(-1, 0, 0).Format("2006-01-02 15:04")
	privJWK := jose.JSONWebKey{
		Key:       privKey,
		KeyID:     timeExpired,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}
	privJSON, err := json.Marshal(privJWK)
	if err != nil {
		t.Fatalf("Failed to marshal JWK: %v", err)
	}
	err = os.WriteFile(jwkFilePath, privJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write JWK file: %v", err)
	}
	// Parse the certificate from tlsCert
	cert, err := x509.ParseCertificate(certs.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	pubJWK := jose.JSONWebKey{
		Key:          cert.PublicKey,
		KeyID:        timeExpired,
		Algorithm:    string(jose.EdDSA),
		Use:          "sig",
		Certificates: []*x509.Certificate{cert},
	}
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{pubJWK},
	}

	jwksJSON, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}
	err = os.WriteFile(jwksFilePath, jwksJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write JWKS file: %v", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"signature-private": privJSON,
			"signature-public":  jwksJSON,
			"test1":             []byte("test1"),
			"test2":             []byte("test2"),
		},
	}
	_, err = clientset.CoreV1().Secrets("kube-system").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}
	return tempDir
}
func TestRenewSignatureCerts(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 1)
	err := renewSignatureCerts(tempDir, clientset)
	if err != nil {
		t.Fatalf("renewSignatureCerts failed: %v", err)
	}

	jwksFilePath := filepath.Join(tempDir, SignaturePublicJWKS)
	updatedJwksData, err := os.ReadFile(jwksFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated JWKS file: %v", err)
	}
	var updatedJWKS jose.JSONWebKeySet
	if err := json.Unmarshal(updatedJwksData, &updatedJWKS); err != nil {
		t.Fatalf("Failed to unmarshal updated JWKS: %v", err)
	}

	if len(updatedJWKS.Keys) != 2 {
		t.Fatalf("Unexpected number of keys in JWKS after renewal: expected 2, got %d", len(updatedJWKS.Keys))
	}
	//// For deep debug
	// fmt.Printf("JWK: %s\n", privJSON)
	// fmt.Printf("JWKS: %s\n", jwksJSON)
	// fmt.Printf("Updated JWKS: %s\n", updatedJwksData)
	// for i, key := range updatedJWKS.Keys {
	// 	fmt.Printf("Key %d:\n", i)
	// 	for j, cert := range key.Certificates {
	// 		fmt.Printf("  Certificate %d:\n", j)
	// 		fmt.Printf("    Subject: %s\n", cert.Subject)
	// 		fmt.Printf("    NotBefore: %s\n", cert.NotBefore)
	// 		fmt.Printf("    NotAfter: %s\n", cert.NotAfter)
	// 	}
	// }

	newCert := updatedJWKS.Keys[1].Certificates[0]
	if time.Until(newCert.NotAfter) < 364*24*time.Hour {
		t.Fatal("The certificate was not renewed as the expiration date is less than a year")
	}

}

func TestRenewSignatureCertsTrimJWKS(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 1)
	jwksFilePath := filepath.Join(tempDir, SignaturePublicJWKS)
	var keySet jose.JSONWebKeySet
	var jwkJSON []byte
	// generate and add 5 certificates in jwks
	for i := 0; i <= 5; i++ {

		certs, err := generateTLSCertificate(fmt.Sprintf("signature-%d", i), "apiserver", CertKeyTypeED25519, 1)
		if err != nil {
			t.Fatalf("generateTLSCertificate failed: %v", err)
		}
		timeExpired := time.Now().AddDate(-(i), 0, 0).Format("2006-01-02 15:04")

		privJWK := jose.JSONWebKey{
			Key:       certs.PrivateKey,
			KeyID:     timeExpired,
			Algorithm: string(jose.EdDSA),
			Use:       "sig",
		}
		jwkJSON, err = json.MarshalIndent(privJWK, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		// Create the public JWK from the certificate
		var pubJWK jose.JSONWebKey
		if len(certs.Certificate) > 0 {
			cert, err := x509.ParseCertificate(certs.Certificate[0])
			if err != nil {
				t.Fatalf("Failed to parse certificate: %v", err)
			}
			pubJWK = jose.JSONWebKey{
				Key:          cert.PublicKey,
				KeyID:        timeExpired,
				Algorithm:    string(jose.EdDSA),
				Use:          "sig",
				Certificates: []*x509.Certificate{cert},
			}
		} else {
			t.Fatalf("Failed to parse certificate: %v", err)
		}

		keySet.Keys = append(keySet.Keys, pubJWK)

	}

	// // For deep debug
	// for i, key := range keySet.Keys {
	// 	fmt.Printf("Key %d:\n", i)
	// 	for j, cert := range key.Certificates {
	// 		fmt.Printf("  Certificate %d:\n", j)
	// 		fmt.Printf("    Subject: %s\n", cert.Subject)
	// 		fmt.Printf("    NotBefore: %s\n", cert.NotBefore)
	// 		fmt.Printf("    NotAfter: %s\n", cert.NotAfter)
	// 	}
	// }
	// //

	jwksJSON, err := json.MarshalIndent(keySet, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	err = os.WriteFile(jwksFilePath, jwksJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write JWKS to file: %v", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"signature-public":  jwksJSON,
			"signature-private": jwkJSON,
		},
	}
	_, err = clientset.CoreV1().Secrets("kube-system").Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}
	renewSignatureCerts(tempDir, clientset)

	jwksData, err := os.ReadFile(jwksFilePath)
	if err == nil {
		if err := json.Unmarshal(jwksData, &keySet); err != nil {
			t.Fatalf("Failed to parse existing JWKS: %v", err)
		}
	}
	if len(keySet.Keys) != 5 {
		t.Fatalf("Unexpected number of keys in JWKS after renewal: expected 5, got %d", len(keySet.Keys))
	}

	// // For deep debug
	// for i, key := range keySet.Keys {
	// 	fmt.Printf("Key %d:\n", i)
	// 	for j, cert := range key.Certificates {
	// 		fmt.Printf("  Certificate %d:\n", j)
	// 		fmt.Printf("    Subject: %s\n", cert.Subject)
	// 		fmt.Printf("    NotBefore: %s\n", cert.NotBefore)
	// 		fmt.Printf("    NotAfter: %s\n", cert.NotAfter)
	// 	}
	// }
	// //

}

func TestRegenerateFilesAfterSignatureSecretChange(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 1)
	jwksFilePath := filepath.Join(tempDir, SignaturePublicJWKS)
	jwkFilePath := filepath.Join(tempDir, SignaturePrivateJWK)
	var keySet jose.JSONWebKeySet

	certs, err := generateTLSCertificate("signature", "apiserver", CertKeyTypeED25519, 365)
	if err != nil {
		t.Fatalf("generateTLSCertificate failed: %v", err)
	}
	timeExpired := time.Now().AddDate(+1, 0, 0).Format("2006-01-02 15:04")

	// Create the public JWK from the certificate
	var pubJWK jose.JSONWebKey
	if len(certs.Certificate) > 0 {
		cert, err := x509.ParseCertificate(certs.Certificate[0])
		if err != nil {
			t.Fatalf("Failed to parse certificate: %v", err)
		}
		pubJWK = jose.JSONWebKey{
			Key:          cert.PublicKey,
			KeyID:        timeExpired,
			Algorithm:    string(jose.EdDSA),
			Use:          "sig",
			Certificates: []*x509.Certificate{cert},
		}
	} else {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	keySet.Keys = append(keySet.Keys, pubJWK)

	privJWK := jose.JSONWebKey{
		Key:       certs.PrivateKey,
		KeyID:     timeExpired,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}
	jwkJSON, err := json.MarshalIndent(privJWK, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	// // For deep debug
	// for i, key := range keySet.Keys {
	// 	fmt.Printf("Key %d:\n", i)
	// 	for j, cert := range key.Certificates {
	// 		fmt.Printf("  Certificate %d:\n", j)
	// 		fmt.Printf("    Subject: %s\n", cert.Subject)
	// 		fmt.Printf("    NotBefore: %s\n", cert.NotBefore)
	// 		fmt.Printf("    NotAfter: %s\n", cert.NotAfter)
	// 	}
	// }
	// //

	jwksJSON, err := json.MarshalIndent(keySet, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"signature-public":  jwksJSON,
			"signature-private": jwkJSON,
		},
	}
	_, err = clientset.CoreV1().Secrets("kube-system").Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}
	renewSignatureCerts(tempDir, clientset)
	jwksNewData, err := os.ReadFile(jwksFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated JWKS file: %v", err)
	}
	jwkNewData, err := os.ReadFile(jwkFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated JWKS file: %v", err)
	}
	if string(jwksNewData) != string(jwksJSON) {
		t.Fatalf("JWKS has not been updated")
	}
	if string(jwkNewData) != string(jwkJSON) {
		t.Fatalf("JWK has not been updated")
	}

}

func TestNoRecordsInSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 365)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"test":  []byte("test"),
			"test2": []byte("test2"),
		},
	}
	_, err := clientset.CoreV1().Secrets("kube-system").Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}
	renewSignatureCerts(tempDir, clientset)
	updatedSecret, err := clientset.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-pki", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}
	if updatedSecret.Data["signature-private"] == nil || updatedSecret.Data["signature-public"] == nil {
		t.Fatalf("Secret has not been updated")
	}
	if updatedSecret.Data["test"] == nil || updatedSecret.Data["test2"] == nil {
		t.Fatalf("Secret updated incorrectly")
	}
}

func TestNoFilesSecretExist(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 365)
	os.Remove(filepath.Join(tempDir, SignaturePrivateJWK))
	os.Remove(filepath.Join(tempDir, SignaturePublicJWKS))
	err := renewSignatureCerts(tempDir, clientset)
	if err != nil {
		t.Fatalf("Failed to renew signature certs: %v", err)
	}
	_, err1 := os.ReadFile(filepath.Join(tempDir, SignaturePrivateJWK))
	_, err2 := os.ReadFile(filepath.Join(tempDir, SignaturePublicJWKS))
	if err1 == os.ErrNotExist || err2 == os.ErrNotExist {
		t.Fatalf("Files have not been created")
	}

}

func TestNoFilesNoSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := t.TempDir()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"test":  []byte("test"),
			"test2": []byte("test2"),
		},
	}
	_, err := clientset.CoreV1().Secrets("kube-system").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}
	err = renewSignatureCerts(tempDir, clientset)
	if err != nil {
		t.Fatalf("Failed to renew signature certs: %v", err)
	}
	_, err1 := os.ReadFile(filepath.Join(tempDir, SignaturePrivateJWK))
	_, err2 := os.ReadFile(filepath.Join(tempDir, SignaturePublicJWKS))
	if err1 == os.ErrNotExist || err2 == os.ErrNotExist {
		t.Fatalf("Files have not been created")
	}
	updatedSecret, err := clientset.CoreV1().Secrets("kube-system").Get(context.TODO(), "d8-pki", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}
	if updatedSecret.Data["signature-private"] == nil || updatedSecret.Data["signature-public"] == nil {
		t.Fatalf("Secret has not been updated")
	}
	if updatedSecret.Data["test"] == nil || updatedSecret.Data["test2"] == nil {
		t.Fatalf("Secret updated incorrectly")
	}
}

func TestEmptyFilesOnDisk(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 1)
	jwksFilePath := filepath.Join(tempDir, SignaturePublicJWKS)
	jwkFilePath := filepath.Join(tempDir, SignaturePrivateJWK)

	os.WriteFile(jwksFilePath, []byte{}, 0o600)
	os.WriteFile(jwkFilePath, []byte{}, 0o600)

	err := renewSignatureCerts(tempDir, clientset)
	if err != nil {
		t.Fatalf("Failed to renew signature certs: %v", err)
	}
	jwksNewData, err := os.ReadFile(jwksFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated JWKS file: %v", err)
	}
	jwkNewData, err := os.ReadFile(jwkFilePath)
	if err != nil {
		t.Fatalf("Failed to read updated JWKS file: %v", err)
	}
	if string(jwksNewData) == "" {
		t.Fatalf("JWKS has not been updated")
	}
	if string(jwkNewData) == "" {
		t.Fatalf("JWK has not been updated")
	}

}

func TestWrongDataInSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	tempDir := GenerateTmpSignatureCerts(t, clientset, 1)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "d8-pki",
		},
		Data: map[string][]byte{
			"signature-public":  []byte("wrong_data"),
			"signature-private": []byte("wrong_data"),
		},
	}
	_, err := clientset.CoreV1().Secrets("kube-system").Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	err = renewSignatureCerts(tempDir, clientset)
	if err == nil {
		t.Fatalf("Renew should be failed")
	} else {
		println("Got expected error: " + err.Error())
	}
}

func TestRegularAPIServerChecksumPaths(t *testing.T) {
	const pkiPath = "/tmp/TestRegularAPIServerChecksumPaths"
	r := RegularRenewer{
		kubernetesPkiPath: pkiPath,
	}

	paths := r.APIServerChecksumPaths()

	if len(paths) != 2 {
		t.Fatalf("Regular renewer APIServerChecksumPaths should contains 2 paths, got %d", len(paths))
	}

	expected := map[string]struct{}{
		fmt.Sprintf("/tmp/TestRegularAPIServerChecksumPaths/%s", "signature-private.jwk"): {},
		fmt.Sprintf("/tmp/TestRegularAPIServerChecksumPaths/%s", "signature-public.jwks"): {},
	}

	for _, p := range paths {
		_, ok := expected[p]
		if !ok {
			t.Fatalf("Regular renewer APIServerChecksumPaths %s not in expected %v", p, expected)
		}
	}
}

func renewSignatureCerts(tmpDir string, k8sInterface kubernetes.Interface) error {
	r := RegularRenewer{
		kubernetesPkiPath: tmpDir,
	}

	return r.Renew(k8sInterface)
}
