/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"strings"
)

type indexTxtLine struct {
	Flag              string
	ExpirationDate    string
	RevocationDate    string
	SerialNumber      string
	Filename          string
	DistinguishedName string
	CommonName        string
}

type clientSecret struct {
	commonName string
	serial     string
	cert       string
	key        string
	revokedAt  string
}

const (
	secretCA         = "openvpn-pki-ca"
	secretServer     = "openvpn-pki-server"
	secretClientTmpl = "openvpn-pki-%s"
	secretDHandTA    = "openvpn-pki-dh-and-ta"
	certFileName     = "tls.crt"
	privKeyFileName  = "tls.key"
	easyrsaMigrated  = "easyrsa-migrated"
)

const easyrsaDir = "/mnt/easyrsa"
const namespace = "d8-openvpn"

func main() {
	config, _ := rest.InClusterConfig()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err)
	}

	_, err = kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), easyrsaMigrated, metav1.GetOptions{})
	if err == nil {
		log.Info("migration is already done")
		return
	}

	indexTxtFile, err := ioutil.ReadFile(easyrsaDir + "/pki/index.txt")
	if err != nil {
		log.Error(err)
	}

	caCertFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/ca.crt", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	caKeyFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/private/ca.key", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	data := map[string]string{
		certFileName:    string(caCertFile),
		privKeyFileName: string(caKeyFile),
	}
	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretCA,
		},
		StringData: data,
		Type:       v1.SecretTypeTLS,
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err == nil {
		log.Infof("secret created (%s)", secretCA)
	} else {
		log.Errorf("error create secret: %s", err.Error())
	}

	serverCertFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/issued/server.crt", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	serverKeyFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/private/server.key", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	data = map[string]string{
		certFileName:    string(serverCertFile),
		privKeyFileName: string(serverKeyFile),
	}
	secret = &v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretServer,
		},
		StringData: data,
		Type:       v1.SecretTypeTLS,
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err == nil {
		log.Infof("secret created (%s)", secretCA)
	} else {
		log.Errorf("error create secret: %s", err.Error())
	}

	taKeyFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/ta.key", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	dhFile, err := ioutil.ReadFile(fmt.Sprintf("%s/pki/dh.pem", easyrsaDir))
	if err != nil {
		log.Error(err)
	}
	data = map[string]string{
		"ta.key": string(taKeyFile),
		"dh.pem": string(dhFile),
	}
	secret = &v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretDHandTA,
		},
		StringData: data,
		Type:       v1.SecretTypeOpaque,
	}
	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err == nil {
		log.Infof("secret created (%s)", secretCA)
	} else {
		log.Errorf("error create secret: %s", err.Error())
	}

	indexTxt := indexTxtParser(string(indexTxtFile))

	for _, cert := range indexTxt {
		fmt.Println(cert.CommonName)
		if cert.CommonName == "server" {
			continue
		}

		var s clientSecret

		path := fmt.Sprintf("%s/pki/issued/%s.crt", easyrsaDir, cert.CommonName)
		if !checkFileExists(path) {
			log.Printf("file not found: %s", path)
			path = fmt.Sprintf("%s/pki/revoked/certs_by_serial/%s.crt", easyrsaDir, cert.SerialNumber)
		}
		file, err := ioutil.ReadFile(path)
		if err != nil {
			log.Error(err)
		}
		s.cert = string(file)

		path = fmt.Sprintf("%s/pki/private/%s.key", easyrsaDir, cert.CommonName)
		if !checkFileExists(path) {
			log.Printf("file not found: %s", path)
			path = fmt.Sprintf("%s/pki/revoked/private_by_serial/%s.key", easyrsaDir, cert.SerialNumber)
		}
		file, err = ioutil.ReadFile(path)
		if err != nil {
			log.Error(err)
		}
		s.key = string(file)

		if cert.Flag == "R" {
			s.revokedAt = cert.RevocationDate
		}

		if cert.Flag != "V" && cert.Flag != "R" {
			log.Errorf("unknown flag: %s", cert.Flag)
		}

		data := map[string]string{
			certFileName:    s.cert,
			privKeyFileName: s.key,
		}

		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf(secretClientTmpl, strings.ToLower(cert.SerialNumber)),
				Annotations: map[string]string{
					"commonName": cert.CommonName,
					"revokedAt":  cert.RevocationDate,
					"serial":     cert.SerialNumber,
				},
				Labels: map[string]string{
					"name":                         cert.CommonName,
					"type":                         "clientAuth",
					"index.txt":                    "",
					"app.kubernetes.io/managed-by": "ovpn-admin",
				},
			},
			StringData: data,
			Type:       v1.SecretTypeTLS,
		}
		_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err == nil {
			log.Infof("secret created (%s)", cert.CommonName)
		} else {
			log.Errorf("error create secret: %s", err.Error())
		}
	}

	secret = &v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: easyrsaMigrated,
		},
		Type: v1.SecretTypeOpaque,
	}

	_, err = kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("error create secret: %s", easyrsaMigrated)
	}

}

func indexTxtParser(txt string) []indexTxtLine {
	var indexTxt []indexTxtLine

	txtLinesArray := strings.Split(txt, "\n")

	for _, v := range txtLinesArray {
		str := strings.Fields(v)
		if len(str) > 0 {
			switch {
			// case strings.HasPrefix(str[0], "E"):
			case strings.HasPrefix(str[0], "V"):
				indexTxt = append(indexTxt, indexTxtLine{Flag: str[0], ExpirationDate: str[1], SerialNumber: str[2], Filename: str[3], DistinguishedName: str[4], CommonName: str[4][strings.Index(str[4], "=")+1:]})
			case strings.HasPrefix(str[0], "R"):
				indexTxt = append(indexTxt, indexTxtLine{Flag: str[0], ExpirationDate: str[1], RevocationDate: str[2], SerialNumber: str[3], Filename: str[4], DistinguishedName: str[5], CommonName: str[5][strings.Index(str[5], "=")+1:]})
			}
		}
	}

	return indexTxt
}

func checkFileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
