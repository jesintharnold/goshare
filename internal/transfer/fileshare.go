package transfer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"goshare/internal/store"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/quic-go/quic-go"
	"golang.org/x/exp/rand"
)

const (
	clientcertDIR string = "CERT"
	QUIC_PORT     int    = 42425
)

// Listen for connections
// If exist , check whether the consent is there, if not send as no consent is there
// + Look for file consent , if exists then proceed to add the quic connection and recive the file please

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

//QSENDER  - We need to send the status of files to here
