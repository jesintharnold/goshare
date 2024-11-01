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
	Ctx      context.Context
}

func (ts *TransferService) ConnectToPeer() {
	peeraddress := fmt.Sprintf("%s:%d", ts.Peerinfo.IPAddress, ts.Peerinfo.Port)
	conn, err := net.Dial("tcp", peeraddress)
	if err != nil {
		log.Printf("Failed to connect to peer : %v \n,error - %v", peeraddress, err)
		return
	}

	log.Printf("Successfully connected to the peer : %v", peeraddress)
	defer conn.Close()

	fileCon := &FileTransferSession{
		Conn: conn,
		Ctx:  ts.Ctx,
		Role: SENDER,
	}

	ts.Session = append(ts.Session, fileCon)
	fmt.Printf("File Transfer struct %v", &ts.Session)

	consentService := &consent.ConsentService{Conn: conn}
	consentMsg := consent.ConsentMessage{
		Type: consent.INITIAL_CONNECTION,
		Metadata: map[string]string{
			"Name": "Allow me mother fucker",
		},
	}
	consentService.RequestConsent(&consentMsg)
	<-ts.Ctx.Done()
	log.Println("Context cancelled, closing connection")
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
		select {
		case <-ts.Ctx.Done():
			log.Println("Received stop signal, shutting down listener")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
			log.Printf("Connection accepted from %v", conn.RemoteAddr())
			go func(conn net.Conn) {
				defer conn.Close()
				consentService := &consent.ConsentService{Conn: conn}
				consentService.HandleIncomingConsent()
			}(conn)
		}
	}
}
