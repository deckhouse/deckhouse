package seaweedfs

import (
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/tls/certprovider/pemfile"
	"google.golang.org/grpc/security/advancedtls"
)

func DialOptionWithTLS(certFileName, keyFileName, caFileName string) grpc.DialOption {
	// If certificate files are not provided, use insecure connection
	if certFileName == "" || keyFileName == "" || caFileName == "" {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Creating client certificate provider
	clientOptions := pemfile.Options{
		CertFile:        certFileName,
		KeyFile:         keyFileName,
		RefreshDuration: time.Hour, // Set the desired refresh interval
	}
	clientProvider, err := pemfile.NewProvider(clientOptions)
	if err != nil {
		log.Printf("pemfile.NewProvider(%v) failed %v", clientOptions, err)
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Creating root certificate provider
	clientRootOptions := pemfile.Options{
		RootFile:        caFileName,
		RefreshDuration: time.Hour, // Set the desired refresh interval
	}
	clientRootProvider, err := pemfile.NewProvider(clientRootOptions)
	if err != nil {
		log.Printf("pemfile.NewProvider(%v) failed: %v", clientRootOptions, err)
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Configuring client parameters
	options := &advancedtls.ClientOptions{
		IdentityOptions: advancedtls.IdentityCertificateOptions{
			IdentityProvider: clientProvider,
		},
		VerifyPeer: func(params *advancedtls.VerificationFuncParams) (*advancedtls.VerificationResults, error) {
			return &advancedtls.VerificationResults{}, nil
		},
		RootOptions: advancedtls.RootCertificateOptions{
			RootProvider: clientRootProvider,
		},
		VType: advancedtls.CertVerification,
	}

	// Creating ClientCreds object
	ta, err := advancedtls.NewClientCreds(options)
	if err != nil {
		log.Printf("advancedtls.NewClientCreds(%v) failed: %v", options, err)
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	return grpc.WithTransportCredentials(ta)
}

func DialOptionWithoutTLS() grpc.DialOption {
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}
