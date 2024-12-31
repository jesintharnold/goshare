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
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"golang.org/x/exp/rand"
)

const (
	clientcertDIR string = "CERT"
	QUIC_PORT     int    = 42425
)

type QListener struct {
}

func (q *QListener) QUICListener(ctx context.Context) error {
	log.Println("QUIC listener started, Waiting for incoming connections...")
	listenAddr := fmt.Sprintf(":%d", QUIC_PORT)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // For testing only
	}

	listener, err := quic.ListenAddr(listenAddr, tlsConfig, nil)
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
			log.Printf("Failed to accept QUIC connection: %v", err)
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
	metaBuffer := make([]byte, 1024)
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

	fileBuffer := make([]byte, 1024*1024*1)
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

func (q *QListener) createFile(filename string) (*os.File, error) {
	filePath := "C:\\Users\\jesin\\Downloads"
	extenstion := filepath.Ext(filename)
	basename := filepath.Base(filename)
	filename = basename[:len(basename)-len(extenstion)]

	file, err := os.Create(filepath.Join(filePath, filename))
	if err == nil {
		return file, nil
	}
	retry_count := 5
	for retry_count > 0 {
		newName := fmt.Sprintf("%s-%d%s", filename, rand.Intn(10000), extenstion)
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

func (q *QSender) getConnection(ipaddress string) quic.Connection {
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		log.Println(err)
	}
	if peer == nil {
		peer := &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			log.Printf("Failed to add peer to peer manager: %v", err)
		}
	} else if peer.QuicConn != nil {
		return peer.QuicConn
	}
	peeraddress := fmt.Sprintf("%s:%d", ipaddress, QUIC_PORT)
	certificate, err := tls.LoadX509KeyPair(filepath.Join(clientcertDIR, "client.crt"), filepath.Join(clientcertDIR, "client.key"))
	if err != nil {
		log.Printf("Erro loading certificates : %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true,
	}

	quicConfig := &quic.Config{
		KeepAlivePeriod: 10 * time.Second,
		MaxIdleTimeout:  30 * time.Second,
	}
	conn, err := quic.DialAddr(context.Background(), peeraddress, tlsConfig, quicConfig)

	if err != nil {
		log.Printf("Error while attempting to connect to file share QUIC : %s  %v", peeraddress, err)
		return nil
	}

	log.Printf("Successfully connected to the QUIC peer - %s", peeraddress)

	peer.QuicConn = conn
	return conn
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

	conn := q.getConnection(ipaddress)
	fmt.Fprintf(os.Stdout, "Sending file - %s , to the client - %s", filePath, ipaddress)
	file, err := os.Open(filePath)

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
