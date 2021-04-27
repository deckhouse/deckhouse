package d8crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

type Keypair struct {
	Pub  string `json:"pub"`
	Priv string `json:"priv"`
}

func GenKeypair() error {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	privateBytes, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	privateBlock := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: privateBytes,
	}
	privatePEM := pem.EncodeToMemory(privateBlock)

	publicBytes, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}
	publicBlock := &pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: publicBytes,
	}
	publicPEM := pem.EncodeToMemory(publicBlock)

	keypairJSON, err := json.Marshal(Keypair{
		Pub:  string(publicPEM),
		Priv: string(privatePEM),
	})
	if err != nil {
		return err
	}

	fmt.Println(string(keypairJSON))
	return nil
}
