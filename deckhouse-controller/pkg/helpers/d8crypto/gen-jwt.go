package d8crypto

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	jose "github.com/square/go-jose/v3"
)

type payloadMap map[string]interface{}

func GenJWT(privateKeyPath string, claims map[string]string, ttl time.Duration) error {
	pubPem, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(pubPem)
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	signerKey := jose.SigningKey{Algorithm: jose.EdDSA, Key: key}
	var signerOpts = jose.SignerOptions{}
	tokenSigner, err := jose.NewSigner(signerKey, &signerOpts)
	if err != nil {
		return err
	}

	tokenClaims := payloadMap{}
	for key, value := range claims {
		tokenClaims[key] = value
	}
	tokenClaims["nbf"] = time.Now().UTC().Unix()
	tokenClaims["exp"] = time.Now().Add(ttl).UTC().Unix()

	tokenClaimsBytes, err := json.Marshal(tokenClaims)
	if err != nil {
		return err
	}

	tokenSignature, err := tokenSigner.Sign(tokenClaimsBytes)
	if err != nil {
		return err
	}

	tokenString, err := tokenSignature.CompactSerialize()
	if err != nil {
		return err
	}

	fmt.Println(tokenString)
	return nil
}
