package transfer

import (
	"context"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/discovery"
	"goshare/internal/fileshare"
	"log"
	"net"
	"strings"
	"sync"
)

type PeerConnection struct {
	Peerinfo discovery.PeerInfo
	Ctx      context.Context
	consent  *consent.Consent
	cancel   context.CancelFunc
	filecon  *fileshare.Fileshare
	tcpcon   net.Conn
}

func (ts *PeerConnection) ConnectToPeer() {
	peeraddress := fmt.Sprintf("%s:%d", ts.Peerinfo.IPAddress, ts.Peerinfo.Port)
	log.Printf("Trying to connect to peer on address - %s", peeraddress)
	conn, err := net.Dial("tcp", peeraddress)
	log.Println(conn)
	if err != nil {
		log.Printf("Failed to connect to peer : %v \n,error - %v", peeraddress, err)
		return
	}

	ts.tcpcon = conn
	log.Printf("Successfully connected to the peer : %v", peeraddress)
	defer conn.Close()

	//create a new context for each connection
	ctx, cancel := context.WithCancel(context.Background())

	//Add initial conenctions , contexts and cancel functions and peer info
	ts.consent = consent.NewConsent(conn, ctx)
	ts.Ctx = ctx
	ts.cancel = cancel

	consentMsg := consent.ConsentMessage{
		Type: consent.INITIAL_CONNECTION,
		Metadata: map[string]string{
			"name": fmt.Sprintf("%s with %s want to Initate file share", strings.ToUpper(ts.Peerinfo.Name), ts.Peerinfo.IPAddress),
		},
	}

	res, err := ts.consent.RequestConsent(&consentMsg)
	if err != nil {
		log.Println(err)
		return
	}

	//Initate the consent for QUIC connection
	if res {
		log.Println("Handshake consent is given , Initating a QUIC protocol connection for file share")
		fs := fileshare.NewFileshare(ctx)
		ts.filecon = fs

		//LOOK FOR READINESS_SIGNAL then proceed for this
		conMsg, res := ts.consent.HandleIncomingConsent()
		log.Printf("Readiness Notification %v", consentMsg)
		if res && conMsg.Type == consent.READINESS_NOTIFICATION {
			ts.filecon.ConnectPeer(ts.Peerinfo.IPAddress)
		}
	}

	<-ts.Ctx.Done()
	log.Println("Context cancelled, closing connection")
}

func (ts *PeerConnection) HandleIncomingCon() {
	// defer ts.tcpcon.Close()

	// peeraddress := ts.tcpcon.RemoteAddr().String()
	// quic_remote_address, _, err := net.SplitHostPort(peeraddress)
	// if err != nil {
	// 	fmt.Printf("Error while extracting IP address for quic connection %v", err)
	// }
	ctx, cancel := context.WithCancel(context.Background())
	ts.consent = consent.NewConsent(ts.tcpcon, ctx)
	ts.cancel = cancel
	resMsg, resConsent := ts.consent.HandleIncomingConsent()
	if resConsent && resMsg.Type == consent.INITIAL_CONNECTION {
		ts.filecon = fileshare.NewFileshare(ctx)
		// listener, err := ts.filecon.ListenPeer(quic_remote_address, ctx)
		// if err != nil {
		// 	fmt.Printf("Error while listening for QUIC connection: %v", err)
		// 	return
		// }
		// if listener {
		ts.consent.NotifyReadiness()
		// }
	}

	// defer func() {
	// 	if ts.tcpcon != nil {
	// 		ts.tcpcon.Close()
	// 	}
	// }()
}

func NewPeerConnection(id string, name string, ipaddress string, conn net.Conn) *PeerConnection {
	return &PeerConnection{
		Peerinfo: discovery.PeerInfo{
			ID:        id,
			Name:      name,
			IPAddress: ipaddress,
			Port:      42424,
		},
		tcpcon: conn,
	}
}

//Creating a peer manager

type PeerManager struct {
	activepeers map[string]*PeerConnection
	peerlock    sync.Mutex
}

func (pm *PeerManager) ListenToPeer() {
	const port = 42424
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err != nil {
		log.Printf("Failed to start listener on port: %v", err)
	}
	defer listener.Close()
	log.Printf("Listening for incoming connections on port -  %d", port)
	for {
		select {
		case <-ctx.Done():
			log.Println("Received stop signal, shutting down listener")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
			log.Printf("tcp connection accepted from %v", conn.RemoteAddr())

			ipaddress, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				fmt.Printf("Error while extracting IP address for quic connection %v", err)
			}

			//Create a new PeerConnection object
			pm.peerlock.Lock()
			pm.activepeers[ipaddress] = NewPeerConnection("123", "jesinth-1", ipaddress, conn)
			pm.peerlock.Unlock()

			go pm.activepeers[ipaddress].HandleIncomingCon()
		}
	}
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		activepeers: make(map[string]*PeerConnection),
	}
}
