package transfer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/store"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"crypto/rand"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	exp "golang.org/x/exp/rand"
)

const (
	clientcertDIR string = "certs"
	QUIC_PORT     int    = 42425
)

type QListener struct {
}

func (q *QListener) QUICListener(ctx context.Context) error {
	log.Println("QUIC listener started, Waiting for incoming connections...")
	listenAddr := fmt.Sprintf(":%d", QUIC_PORT)
	tlsConfig, err := q.generateTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %v", err)
	}

	quicConfig := &quic.Config{
		KeepAlivePeriod: 10 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
		EnableDatagrams: true,
	}

	listener, err := quic.ListenAddr(listenAddr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("error while attempting to listen on QUIC :%v", err)
	}
	go func(listener *quic.Listener) {
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
				log.Println("Received stop signal, shutting down QUIC listener")
				return
			default:
				quiccon, err := listener.Accept(ctx)
				if err != nil {
					log.Printf("Failed to accept QUIC connection: %v", err)
					continue
				}
				log.Printf("Connection accepted from %v", quiccon.RemoteAddr())
				ipaddress, _, err := net.SplitHostPort(quiccon.RemoteAddr().String())
				if err != nil {
					fmt.Printf("Error while extracting IP address for quic connection %v", err)
				}

				// After accepting connecting connection now we need to look for new streams
				go func(quiccon quic.Connection, ip string) {
					defer quiccon.CloseWithError(0, "connection closed")
					q.handleIncomingStreams(quiccon, ip, ctx)
				}(quiccon, ipaddress)

			}
		}
	}(listener)
	return nil
}

func (q *QListener) handleIncomingStreams(quiccon quic.Connection, ip string, ctx context.Context) {
	for {
		stream, err := quiccon.AcceptStream(ctx)
		log.Printf("Accepting QUIC connections, Address - %s Stream - %v", quiccon.RemoteAddr(), stream.StreamID())
		log.Printf("QUIC Identified IP - %s", ip)
		if err != nil {
			log.Printf("Failed to accept QUIC connection: %v", err.Error())
			continue
		}

		err = q.receiveFile(stream, ip)
		if err != nil {
			log.Printf("Error during file receive - %s, %v", quiccon.RemoteAddr(), stream.StreamID())

		}
	}
}

func (q *QListener) receiveFile(stream quic.Stream, ip string) error {
	defer stream.Close()
	metaBuffer := make([]byte, 4096)
	metastreamsize, err := stream.Read(metaBuffer)
	if err != nil && err != io.EOF {
		log.Printf("Error reading metadata: %v", err)
		return err
	}

	peerinstance, err := store.Getpeermanager().Getpeer(ip)
	if err != nil {
		return fmt.Errorf("error from quic receive files %v", err)
	}

	//Decoding the metadata here
	var metadata store.FileInfo
	if err := json.Unmarshal(metaBuffer[:metastreamsize], &metadata); err != nil {
		log.Printf("Error unmarshalling metadata : %v", err)
		return err
	}
	log.Printf("Receiving file: %s, size: %d bytes", metadata.Filename, metadata.Size)

	transferkey := peerinstance.FileSession.CreateTransfer(metadata, store.RECEIVING)

	//Create a file based on metadata recived
	file, err := q.createFile(metadata.Filename)
	if err != nil {
		log.Printf("Error creating a file: %v", err)
		return err
	}

	fileBuffer := make([]byte, 1024*1024*4)
	var totalbyterec int64
	for {
		n, err := stream.Read(fileBuffer)
		if err == io.EOF {
			peerinstance.FileSession.CompleteTransfer(transferkey)
			break
		}
		if err != nil {
			log.Printf("Error reading file data , reciving file: %v", err)
			peerinstance.FileSession.FailTransfer(transferkey, err)
			return err
		}

		if _, err := file.Write(fileBuffer[:n]); err != nil {
			log.Printf("Error writing to file: %v", err)
			peerinstance.FileSession.FailTransfer(transferkey, err)
			return err
		}

		totalbyterec += int64(n)
		peerinstance.FileSession.UpdateTransferProgress(transferkey, totalbyterec)
	}

	log.Printf("File %s saved successfully -", file.Name())
	return nil
}

func (q *QListener) generateTLSConfig() (*tls.Config, error) {
	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Goshare"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180), // Valid for 180 days
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	// PEM encode the certificate and private key
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Load the certificate
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-file-transfer"},
	}, nil
}

func (q *QListener) createFile(filename string) (*os.File, error) {
	filePath := "//data//projects//goshare"
	extenstion := filepath.Ext(filename)
	basename := filepath.Base(filename)
	filename = basename[:len(basename)-len(extenstion)]

	file, err := os.Create(filepath.Join(filePath, filename))
	if err == nil {
		return file, nil
	}
	retry_count := 3
	for retry_count > 0 {
		newName := fmt.Sprintf("%s-%d%s", filename, exp.Intn(10000), extenstion)
		newFilePath := filepath.Join(filePath, newName)
		file, err := os.Create(newFilePath)

		if err == nil {
			log.Printf("File already exists, created new file with name: %s", newFilePath)
			return file, nil
		}
		retry_count--
		log.Printf("Retry failed. Remaining attempts: %d", retry_count)
	}
	return nil, fmt.Errorf("exceeded retry attempts, could not create file : %s", filename)
}

// QSENDER  - We need to send the status of files to here
type QSender struct {
}

func (q *QSender) GetConnection(ipaddress string) quic.Connection {
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		log.Println(err)
	}
	if peer == nil {
		fmt.Println("Peer does not exist , Adding to Peer Manager")
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			log.Printf("Failed to add peer to peer manager: %v", err)
		}
	} else if peer.QuicConn != nil {
		return *peer.QuicConn
	}
	peeraddress := fmt.Sprintf("%s:%d", ipaddress, QUIC_PORT)
	tlsConfig, err := q.generateTLSConfig()
	if err != nil {
		log.Printf("Failed to generate TLS config: %v", err)
		return nil
	}

	// Add these settings
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.ServerName = ipaddress

	quicConfig := &quic.Config{
		KeepAlivePeriod: 10 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
		EnableDatagrams: true,
	}
	conn, err := quic.DialAddr(context.Background(), peeraddress, tlsConfig, quicConfig)

	if err != nil {
		log.Printf("Error while attempting to connect to file share QUIC : %s  %v", peeraddress, err.Error())
		return nil
	}

	log.Printf("Successfully connected to the QUIC peer - %s", peeraddress)

	peer.QuicConn = &conn
	return conn

}

func (q *QSender) generateTLSConfig() (*tls.Config, error) {
	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Goshare"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180), // Valid for 180 days
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	// PEM encode the certificate and private key
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Load the certificate
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-file-transfer"},
	}, nil
}

func (q *QSender) prevalidation(ipaddress string) bool {
	// This function is to do the pre-validation check before sending file
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		log.Printf("Error getting peer: %v", err)
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
			Consent:     false,
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			log.Printf("Failed to add peer to peer manager: %v", err)
			return false
		}
	}

	if peer.Consent {
		return true
	}
	//we need to provide them with IPAddress and Name bro
	msg := consent.ConsentMessage{
		Type: consent.INITIAL,
		Metadata: map[string]string{
			"name": fmt.Sprintf("%s with %s want to Initate file share", "I am legend", "My IP Address"),
		},
	}
	log.Printf("No consent found , requesting the client %s", ipaddress)
	consent.Getconsent().Sendmessage(ipaddress, &msg)
	return false
}

func (q *QSender) SendFile(ipaddress string, filePath string) error {

	validation := q.prevalidation(ipaddress)

	if !validation {
		log.Printf("No Consent found. Requested %s for consent , try sharing the files after consent", ipaddress)
		return nil
	}

	conn := q.GetConnection(ipaddress)

	if conn == nil {
		fmt.Print("connection is nil")
	}

	fmt.Fprintf(os.Stdout, "Sending file - %s , to the client - %s", filePath, ipaddress)
	file, err := os.Open("F:\\GO_PROJECTS\\goshare\\cmd\\linkedin.png")

	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("File does not exist in the path %s", filePath)
			return nil
		} else {
			log.Printf("Error opening file path %s , %v", filePath, err)
			return err
		}
	}

	filestats, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file info %s: %v", filePath, err)
		return err
	}

	metadata := store.FileInfo{
		Filename: filestats.Name(),
		Size:     filestats.Size(),
	}

	Qstream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("Error creating a stream for sending Metadata %v", err)
	}

	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		return err
	}

	transferkey := peer.FileSession.CreateTransfer(metadata, store.SENDING)

	metaJSON, err := json.Marshal(&metadata)
	if err != nil {
		log.Printf("Error while converting the File metadata for sending %v", err)
		return err
	}

	_, err = Qstream.Write(metaJSON)
	if err != nil {
		log.Printf("Error will sending reading file : %v", err)
		return err
	}
	log.Printf("Sent metadata for %s,%v", filePath, metaJSON)

	// We are sending now actual files bro
	tempfilebuffer := make([]byte, 1*1024*1024)
	var totalbytesent int64
	for {
		n, err := file.Read(tempfilebuffer)
		if err == io.EOF {
			peer.FileSession.CompleteTransfer(transferkey)
			break
		}
		if err != nil {
			log.Printf("Error will sending reading file : %v", err)
			peer.FileSession.FailTransfer(transferkey, err)
			return err
		}
		_, err = Qstream.Write(tempfilebuffer[:n])
		if err != nil {
			log.Printf("Error will sending reading file : %v", err)
			peer.FileSession.FailTransfer(transferkey, err)
			return err
		}

		totalbytesent += int64(n)
		//update total bytes in terms of single byte ot interms of percentage
		peer.FileSession.UpdateTransferProgress(transferkey, totalbytesent)
	}
	return nil
}

// Two singleton process
// 1 - QListener
// 2 - QSender

var (
	QListen   *QListener
	QSend     *QSender
	singleton sync.Once
)

func Getfileshare() (*QListener, *QSender) {
	singleton.Do(func() {
		QListen = &QListener{}
		QSend = &QSender{}
	})

	return QListen, QSend
}
