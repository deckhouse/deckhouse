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
	// Если файлы сертификатов не указаны, используем небезопасное подключение
	if certFileName == "" || keyFileName == "" || caFileName == "" {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Создание провайдера клиентских сертификатов
	clientOptions := pemfile.Options{
		CertFile:        certFileName,
		KeyFile:         keyFileName,
		RefreshDuration: time.Hour, // Установите нужный интервал обновления
	}
	clientProvider, err := pemfile.NewProvider(clientOptions)
	if err != nil {
		log.Printf("pemfile.NewProvider(%v) failed %v", clientOptions, err)
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Создание провайдера корневых сертификатов
	clientRootOptions := pemfile.Options{
		RootFile:        caFileName,
		RefreshDuration: time.Hour, // Установите нужный интервал обновления
	}
	clientRootProvider, err := pemfile.NewProvider(clientRootOptions)
	if err != nil {
		log.Printf("pemfile.NewProvider(%v) failed: %v", clientRootOptions, err)
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	// Настройка параметров клиента
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

	// Создание объекта ClientCreds
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
