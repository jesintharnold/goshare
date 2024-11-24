package fileshare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"crypto/tls"

	"github.com/quic-go/quic-go"
	"golang.org/x/exp/rand"
)

const (
	clientcertDIR string = "CERT"
	QUIC_PORT     int    = 42425 // Different from TCP port 42424
)

type Fileshare struct {
	quiccon        quic.Connection
	ctx            context.Context
	cancel         context.CancelFunc
	sessionmanager *SessionManager
}

func (fs *Fileshare) ConnectPeer(peeraddress string) (*Fileshare, error) {
	peeraddress = fmt.Sprintf("%s:%d", peeraddress, QUIC_PORT)

	certificate, err := tls.LoadX509KeyPair(filepath.Join(clientcertDIR, "client.crt"), filepath.Join(clientcertDIR, "client.key"))
	if err != nil {
		log.Printf("Erro loading certificates : %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}
	conn, err := quic.DialAddr(fs.ctx, peeraddress, tlsConfig, nil)

	if err != nil {
		log.Printf("Error while attempting to connect to file share QUIC : %s  %v", peeraddress, err)
		return nil, err
	}
	log.Printf("Successfully connected to the QUIC peer - %s", peeraddress)
	fs.quiccon = conn
	fs.sessionmanager = NewSession(fs.ctx)
	time.Sleep(30 * time.Second)
	fs.SendFile("C:\\Users\\jesin\\Downloads\\EMI.txt")
	return fs, nil
}
func (fs *Fileshare) SendFile(filePath string) error {

	//get the metadata and sent as packet to client
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file path %s , %v", filePath, err)
		return err
	}
	defer file.Close()

	//get the file stat size
	filestats, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file info %s: %v", filePath, err)
		return err
	}

	metadata := FileInfo{
		Filename: filestats.Name(),
		Size:     filestats.Size(),
	}

	//Simply open a stream to send the file
	stream, err := fs.quiccon.OpenStreamSync(fs.ctx)
	if err != nil {
		log.Printf("Error creating stream to send %v", err)
		return err
	}
	defer stream.Close()

	transferkey := fs.sessionmanager.CreateTransfer(metadata, SENDING)

	//Send the metadata first to the reciver
	// metabuffer := make([]byte, 1024)
	metaJSON, err := json.Marshal(&metadata)
	if err != nil {
		log.Printf("Error while marshalling metadata : %v", err)
		return err
	}

	_, err = stream.Write(metaJSON)
	if err != nil {
		log.Printf("Error will sending reading file : %v", err)
		return err
	}
	log.Printf("Sent metadata for %s,%v", filePath, metaJSON)

	//Create a buffer to temp store the chunks for reading and sending those chunks in QUIC-stream
	//crete a loop to send it until EOF is recived
	tempfilebuffer := make([]byte, 1024*1024*1)
	var totalbytesent int64
	for {
		n, err := file.Read(tempfilebuffer)
		if err == io.EOF {
			fs.sessionmanager.CompleteTransfer(transferkey)
			break
		}
		if err != nil {
			log.Printf("Error will sending reading file : %v", err)
			fs.sessionmanager.FailTransfer(transferkey, err)
			return err
		}

		_, err = stream.Write(tempfilebuffer[:n])
		if err != nil {
			log.Printf("Error will sending reading file : %v", err)
			fs.sessionmanager.FailTransfer(transferkey, err)
			return err
		}
		totalbytesent += int64(n)
		fs.sessionmanager.UpdateTransferProgress(transferkey, totalbytesent)

	}
	return nil

}
func (fs *Fileshare) ListenPeer(peeraddress string, ctx context.Context) (interface{}, error) {
	peeraddress = fmt.Sprintf("%s:%d", peeraddress, QUIC_PORT)
	listenAddr := fmt.Sprintf(":%d", QUIC_PORT)

	log.Printf("Listening for incoming QUIC connections - %s", peeraddress)

	certificate, err := tls.LoadX509KeyPair(filepath.Join(clientcertDIR, "client.crt"), filepath.Join(clientcertDIR, "client.key"))
	if err != nil {
		log.Printf("Error loading certificates : %v", err)
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true, // For testing only
	}
	listener, err := quic.ListenAddr(listenAddr, tlsConfig, nil)
	if err != nil {
		log.Printf("Error while attempting to listen on QUIC : %s  %v", peeraddress, err)
		return nil, err
	}
	log.Printf("Successfully listening to the QUIC peer - %s", peeraddress)

	fs.ctx = ctx
	fs.sessionmanager = NewSession(fs.ctx)
	defer listener.Close()

	//use loop to listen and accept incoming connections
	for {
		select {
		case <-fs.ctx.Done():
			log.Println("Received stop signal, shutting down QUIC listener")
			return nil, nil
		default:
			quiccon, err := listener.Accept(fs.ctx)
			if err != nil {
				log.Printf("Failed to accept QUIC connection: %v", err)
				continue
			}
			log.Printf("Connection accepted from %v", peeraddress)

			// After accepting connecting connection now we need to look for new streams
			go func(quiccon quic.Connection) {
				defer quiccon.CloseWithError(0, "connection closed")
				fs.handleIncomingStreams(quiccon)
			}(quiccon)

		}
	}
}

func (fs *Fileshare) handleIncomingStreams(quiccon quic.Connection) {
	for {
		stream, err := quiccon.AcceptStream(fs.ctx)
		log.Printf("Accepting QUIC connections, Address - %s Stream - %v", quiccon.RemoteAddr(), stream.StreamID())
		if err != nil {
			log.Printf("Failed to accept QUIC connection: %v", err)
			continue
		}
		err = fs.receiveFile(stream)
		if err != nil {
			log.Printf("Error during file receive - %s, %v", quiccon.RemoteAddr(), stream.StreamID())
		}
	}
}

func (fs *Fileshare) receiveFile(stream quic.Stream) error {
	defer stream.Close()
	metaBuffer := make([]byte, 1024)
	metastreamsize, err := stream.Read(metaBuffer)
	if err != nil && err != io.EOF {
		log.Printf("Error reading metadata: %v", err)
		return err
	}
	//Decoding the metadata here
	var metadata FileInfo
	if err := json.Unmarshal(metaBuffer[:metastreamsize], &metadata); err != nil {
		log.Printf("Error unmarshalling metadata : %v", err)
		return err
	}
	log.Printf("Receiving file: %s, size: %d bytes", metadata.Filename, metadata.Size)

	transferkey := fs.sessionmanager.CreateTransfer(metadata, RECEIVING)

	//Create a file based on metadata recived
	file, err := fs.createFile(metadata.Filename)
	if err != nil {
		log.Printf("Error creating a file: %v", err)
		return err
	}

	fileBuffer := make([]byte, 1024*1024*1)
	var totalbyterec int64
	for {
		n, err := stream.Read(fileBuffer)
		if err == io.EOF {
			fs.sessionmanager.CompleteTransfer(transferkey)
			break
		}
		if err != nil {
			log.Printf("Error reading file data , reciving file: %v", err)
			fs.sessionmanager.FailTransfer(transferkey, err)
			return err
		}

		if _, err := file.Write(fileBuffer[:n]); err != nil {
			log.Printf("Error writing to file: %v", err)
			fs.sessionmanager.FailTransfer(transferkey, err)
			return err
		}

		totalbyterec += int64(n)
		fs.sessionmanager.UpdateTransferProgress(transferkey, totalbyterec)
	}

	log.Printf("File %s saved successfully -", file.Name())
	return nil
}

func (fs *Fileshare) createFile(filename string) (*os.File, error) {
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

func NewFileshare(parentCtx context.Context) *Fileshare {
	ctx, cancel := context.WithCancel(parentCtx)
	return &Fileshare{
		ctx:            ctx,
		cancel:         cancel,
		sessionmanager: NewSession(ctx),
	}
}
