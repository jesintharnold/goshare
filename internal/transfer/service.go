package transfer

import (
	"context"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/discovery"
	"log"
	"net"
)

type TransferService struct {
	Peerinfo discovery.PeerInfo
	Session  []*FileTransferSession
}

func (ts *TransferService) ConnectToPeer() {
	peeraddress := fmt.Sprintf("%s:%d", ts.Peerinfo.IPAddress, ts.Peerinfo.Port)
	context, cancel := context.WithCancel(context.Background())
	conn, err := net.Dial("tcp", peeraddress)
	if err != nil {
		log.Printf("Failed to connect to peer : %v \n,error - %v", peeraddress, err)
	}
	log.Printf("Successfully connected to the peer : %v", peeraddress)
	fileCon := &FileTransferSession{
		Conn:   conn,
		Ctx:    context,
		Cancel: cancel,
		Role:   SENDER,
	}

	ts.Session = append(ts.Session, fileCon)
	fmt.Printf("File Transfer struct %v", &ts.Session)
}

func (ts *TransferService) ListenToPeer() {
	const port = 42424
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("Failed to start listener on port: %v", err)
	}
	defer listener.Close()

	log.Printf("Listening for incoming connections on port %d :", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		log.Printf("Connection accepted from %v", conn.RemoteAddr())
		go func() {
			consentService := &consent.ConsentService{Conn: conn}
			consentService.HandleIncomingConsent()
		}()

	}
}
