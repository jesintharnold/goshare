package transfer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/errors"
	"goshare/internal/store"
	"io"
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
	"github.com/schollz/progressbar/v3"
	exp "golang.org/x/exp/rand"
)

const (
	clientcertDIR string = "certs"
	QUIC_PORT     int    = 42425
)

type QListener struct {
}

func (q *QListener) QUICListener(ctx context.Context) error {
	listenAddr := fmt.Sprintf(":%d", QUIC_PORT)
	tlsConfig, err := q.generateTLSConfig()
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.WARNING, "QUIC", "failed to generate TLS config", err)
	}

	quicConfig := &quic.Config{
		KeepAlivePeriod: 10 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
		EnableDatagrams: true,
	}

	listener, err := quic.ListenAddr(listenAddr, tlsConfig, quicConfig)
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "error while attempting to listen on QUIC connection", err)

	}
	go func(listener *quic.Listener) error {
		defer listener.Close()
		for {
			select {
			case <-ctx.Done():
				return errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("shutdown signal recieved - %s", time.Now()), nil)
			default:
				quiccon, err := listener.Accept(ctx)
				if err != nil {
					errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("Failed to accept QUIC connection: %s", quiccon.RemoteAddr().String()), err)
					continue
				}
				ipaddress, _, err := net.SplitHostPort(quiccon.RemoteAddr().String())
				if err != nil {
					errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Error while extracting IP address for quic connection", err)
				}
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
		errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("QUIC connection Address - %s Stream - %v", quiccon.RemoteAddr(), stream.StreamID()), nil)
		if err != nil {
			errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Failed to accept QUIC connection", err)
			continue
		}

		err = q.receiveFile(stream, ip)
		if err != nil {
			fmt.Println(err.Error())
			errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", fmt.Sprintf("Error during file receive - %s, %v \n", quiccon.RemoteAddr(), stream.StreamID()), err)
		}
	}
}

func (q *QListener) receiveFile(stream quic.Stream, ip string) error {
	defer stream.Close()
	metaBuffer := make([]byte, 1024*1024*4)
	metastreamsize, err := stream.Read(metaBuffer)
	if err != nil && err != io.EOF {
		return errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", fmt.Sprintf("Error reading metadata: %v", err.Error()), nil)
	}

	peerinstance, err := store.Getpeermanager().Getpeer(ip)
	if err != nil {
		return errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", fmt.Sprintf("Error reading metadata: %v", err.Error()), nil)
	}

	//Decoding the metadata here
	var metadata store.FileInfo
	if err := json.Unmarshal(metaBuffer[:metastreamsize], &metadata); err != nil {
		return errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", fmt.Sprintf("Error unmarshalling JSON DATA: %v", err.Error()), nil)
	}
	errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", fmt.Sprintf("Receiving file: %s, size: %d bytes", metadata.Filename, metadata.Size), nil)
	transferkey := peerinstance.FileSession.CreateTransfer(metadata, store.RECEIVING)
	file, err := q.createFile(metadata.Filename)
	if err != nil {
		//log.Printf("Error creating a file: %v", err)

		return err
	}

	bar := progressbar.NewOptions64(
		metadata.Size,
		progressbar.OptionSetDescription(fmt.Sprintf("Recieving %s from %s", metadata.Filename, ip)),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	fileBuffer := make([]byte, 1024*1024*4)
	var totalbyterec int64
	for {
		n, err := stream.Read(fileBuffer)
		if err == io.EOF {
			peerinstance.FileSession.CompleteTransfer(transferkey)
			break
		}
		if err != nil {
			peerinstance.FileSession.FailTransfer(transferkey, err)
			return errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", "Error reading file data , reciving file", err)
		}

		if _, err := file.Write(fileBuffer[:n]); err != nil {
			peerinstance.FileSession.FailTransfer(transferkey, err)
			return errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", "Error writing to file", err)
		}

		totalbyterec += int64(n)
		bar.Add(n)
		peerinstance.FileSession.UpdateTransferProgress(transferkey, totalbyterec)
	}
	errors.NewError(errors.ErrFileTransfer, errors.INFO, "QUIC", fmt.Sprintf("File %s saved successfully -", file.Name()), nil)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", "Could not get home directory", err)
	}

	goshareDir := filepath.Join(homeDir, "goshare", "downloads")
	err = os.MkdirAll(goshareDir, 0755)
	if err != nil {
		return nil, errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", "Could not create goshare directory", err)
	}
	filePath := filepath.Join(goshareDir, filename)
	extenstion := filepath.Ext(filename)
	basename := filepath.Base(filename)
	filenamewithoutext := basename[:len(basename)-len(extenstion)]

	file, err := os.Create(filepath.Join(filePath, filename))
	if err == nil {
		return file, nil
	}
	retry_count := 3
	for retry_count > 0 {
		newName := fmt.Sprintf("%s-%d%s", filenamewithoutext, exp.Intn(10000), extenstion)
		newFilePath := filepath.Join(goshareDir, newName)
		file, err := os.Create(newFilePath)

		if err != nil {
			errors.NewError(errors.ErrFileTransfer, errors.INFO, "QUIC", err.Error(), err)
			return nil, err
		} else if err == nil {
			errors.NewError(errors.ErrFileTransfer, errors.INFO, "QUIC", fmt.Sprintf("File already exists, created new file with name: %s", newFilePath), nil)
			return file, nil
		}
		retry_count--
		errors.NewError(errors.ErrFileTransfer, errors.INFO, "QUIC", fmt.Sprintf("Retry failed. Remaining attempts: %d", retry_count), nil)
	}
	return nil, fmt.Errorf("exceeded retry attempts, could not create file : %s", filename)
}

// QSENDER  - We need to send the status of files to here
type QSender struct {
}

func (q *QSender) GetConnection(ipaddress string) quic.Connection {
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		//log.Println(err)
	}
	if peer == nil {
		errors.NewError(errors.ErrFileTransfer, errors.ERROR, "QUIC", "Peer does not exist , Adding to Peer Manager", nil)
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Failed to add peer to peer manager", err)
		}
	} else if peer.QuicConn != nil {
		return *peer.QuicConn
	}
	peeraddress := fmt.Sprintf("%s:%d", ipaddress, QUIC_PORT)
	tlsConfig, err := q.generateTLSConfig()
	if err != nil {
		errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Failed to generate TLS config", err)
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
		errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", fmt.Sprintf("Error while attempting to connect to file share QUIC : %s \n", peeraddress), err)
		return nil
	}
	errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("Successfully connected to the QUIC peer - %s", peeraddress), nil)
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
		//log.Printf("Error getting peer: %v", err)
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
			Consent:     false,
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			//log.Printf("Failed to add peer to peer manager: %v", err)
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
	//log.Printf("No consent found , requesting the client %s", ipaddress)
	consent.Getconsent().Sendmessage(ipaddress, &msg)
	return false
}

func (q *QSender) SendFile(ipaddress string, filePath string) error {

	validation := q.prevalidation(ipaddress)

	if !validation {
		errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("No consent found , requesting consent %s", ipaddress), nil)
		return nil
	}

	conn := q.GetConnection(ipaddress)

	if conn == nil {
		errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", "Connection is nil", nil)
	}

	errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", fmt.Sprintf("Sending file - %s , to the client - %s \n", filePath, ipaddress), nil)

	file, err := os.Open(filePath)

	if err != nil {
		if os.IsNotExist(err) {
			return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", fmt.Sprintf("File does not exist in the path %s", filePath), nil)
		} else {
			return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", fmt.Sprintf("Error opening file path %s", filePath), err)
		}
	}

	filestats, err := file.Stat()
	if err != nil {
		//log.Printf("Error getting file info %s: %v", filePath, err)
		return err
	}

	metadata := store.FileInfo{
		Filename: filestats.Name(),
		Size:     filestats.Size(),
	}

	Qstream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Error creating a stream for sending Metadata", err)
	}

	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Error while fetching data", err)
	}

	transferkey := peer.FileSession.CreateTransfer(metadata, store.SENDING)

	metaJSON, err := json.Marshal(&metadata)
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Error while converting the File metadata for sending", err)
	}

	_, err = Qstream.Write(metaJSON)
	if err != nil {
		return errors.NewError(errors.ErrConnection, errors.ERROR, "QUIC", "Error will sending reading file", err)
	}

	bar := progressbar.NewOptions64(
		filestats.Size(),
		progressbar.OptionSetDescription(fmt.Sprintf("Sending %s to %s", filepath.Base(filePath), ipaddress)),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	tempfilebuffer := make([]byte, 4*1024*1024)
	var totalbytesent int64
	for {
		n, err := file.Read(tempfilebuffer)
		if err == io.EOF {
			peer.FileSession.CompleteTransfer(transferkey)
			bar.Finish()
			break
		}
		if err != nil {
			peer.FileSession.FailTransfer(transferkey, err)
			return errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", "Error will sending reading file", err)
		}
		_, err = Qstream.Write(tempfilebuffer[:n])
		if err != nil {
			peer.FileSession.FailTransfer(transferkey, err)
			return errors.NewError(errors.ErrConnection, errors.INFO, "QUIC", "Error will sending reading file", err)
		}
		totalbytesent += int64(n)
		bar.Add(n)
		peer.FileSession.UpdateTransferProgress(transferkey, totalbytesent)
	}
	return nil
}

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
