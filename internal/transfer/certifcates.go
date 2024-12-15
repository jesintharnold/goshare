package tra

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func GenerateParentCerts() {
	dirName := "PARENT"
	err := os.Mkdir(dirName, 0755)
	if err != nil {
		if os.IsExist(err) {
			log.Printf("Directory %s already exists.\n", dirName)
		} else {
			log.Fatalf("Failed to create directory: %v", err)
		}
	} else {
		log.Printf("Directory %s created successfully.\n", dirName)
	}

	randnum, _ := rand.Int(rand.Reader, big.NewInt(1).Lsh(big.NewInt(1), 64))
	certmetadata := &x509.Certificate{
		SerialNumber: randnum,
		Subject: pkix.Name{
			Organization: []string{"goshare", "goshareCLI"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(180 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	parentCA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to generate CA private key: %v", err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certmetadata, certmetadata, &parentCA.PublicKey, parentCA)
	if err != nil {
		log.Fatalf("Failed to create Parent CA certificate: %v", err)
	}

	caCertFile, err := os.Create(filepath.Join(dirName, "parentCA.crt"))
	if err != nil {
		log.Fatalf("Failed to create PARENT ca.crt: %v", err)
	}
	defer caCertFile.Close()

	caKeyFile, err := os.Create(filepath.Join(dirName, "parent.key"))
	if err != nil {
		log.Fatalf("Failed to create parent.key: %v", err)
	}
	defer caKeyFile.Close()

	err = pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		log.Fatalf("Failed to write Parent CA certificate to file: %v", err)
	}
	log.Println("CA certificate saved to parentCA.crt")

	err = pem.Encode(caKeyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(parentCA),
	})
	if err != nil {
		log.Fatalf("Failed to write key to file: %v", err)
	}
	log.Println("Key saved to parent-ca.key")
}

func GenerateClientCerts() {
	clientcert, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to create client certificate :%v", err)
	}
	randnum, _ := rand.Int(rand.Reader, big.NewInt(1).Lsh(big.NewInt(1), 64))
	certmetadata := &x509.Certificate{
		SerialNumber: randnum,
		Subject: pkix.Name{
			Organization: []string{"goshare", "goshareCLI"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(180 * 24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}
	dirparentName := "PARENT"
	dirName := "CERT"

	//load existing parent key now
	parentcert, err := tls.LoadX509KeyPair(filepath.Join(dirparentName, "parentCA.crt"), filepath.Join(dirparentName, "parent.key"))
	if err != nil {
		log.Fatalf("Failed to load CA certificate: %v", err)
	}

	if err != nil {
		log.Fatalf("Failed to parse Parent CA private key: %v", err)
	}

	clientCert, err := x509.CreateCertificate(rand.Reader, certmetadata, certmetadata, &clientcert.PublicKey, parentcert.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to create Client CA certificate: %v", err)
	}

	//we are saving the keys to client
	err = os.Mkdir("CERT", 0755)
	if err != nil {
		if os.IsExist(err) {
			log.Printf("Directory %s already exists.\n", dirName)
		} else {
			log.Fatalf("Failed to create directory: %v", err)
		}
	} else {
		log.Printf("Directory %s created successfully.\n", dirName)
	}

	clientcrt, err := os.Create(filepath.Join(dirName, "client.crt"))
	if err != nil {
		log.Fatalf("Error while creating client crt ,%v", err)
	}
	defer clientcrt.Close()

	clientkey, err := os.Create(filepath.Join(dirName, "client.key"))
	if err != nil {
		log.Fatalf("Error while creating client key ,%v", err)
	}
	defer clientkey.Close()

	//Now encode the pem key to CA.crt & ca.key
	err = pem.Encode(clientcrt, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCert,
	})

	if err != nil {
		log.Fatalf("Failed to write to Client.crt")
	}
	log.Println("clientCA.crt is created")

	parentPrivateKey, ok := parentcert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		log.Fatalf("Failed to convert parent private key to *rsa.PrivateKey")
	}

	err = pem.Encode(clientkey, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(parentPrivateKey),
	})

	if err != nil {
		log.Fatalf("Failed to write to Client.key")
	}
	log.Println("client.key is created")
}
