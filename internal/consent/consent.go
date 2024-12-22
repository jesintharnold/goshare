package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"goshare/internal/store"
	"io"
	"log"
	"net"
	"sync"

	"github.com/google/uuid"
)

type CONSENT int

const (
	INITIAL CONSENT = iota
	FILE
	ERROR
	INITIALRES
	FILERES
)

type ConsentMessage struct {
	Type     CONSENT
	Metadata map[string]string
}

type Consent struct {
	ctx        context.Context
	cancel     context.CancelFunc
	notifychan chan ConsentMessage
}

const CONSENTPORT = 42424

func (c *Consent) sendconsent(conn net.Conn, msg *ConsentMessage) error {
	msgencoder := json.NewEncoder(conn)
	if err := msgencoder.Encode(msg); err != nil {
		return fmt.Errorf("error while sending consent message to user \n error - %s\nremote address -%s", err, conn.RemoteAddr().String())
	}
	return nil
}

func (c *Consent) Receiveconsent() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", CONSENTPORT))
	log.Println("Started Listening for TCP consents")
	if err != nil {
		return fmt.Errorf("failed to listen for incoming connections on port - %d", CONSENTPORT)
	}
	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("recived context cancel signal, listening stopping on port - %d", CONSENTPORT)
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to listen connection: %v", err)
				continue
			}
			log.Printf("Consent listen to peer - %s", conn.RemoteAddr().String())
			ipaddress, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				fmt.Printf("Error while extracting IP address for quic connection %v", err)
			}
			peer, err := store.Getpeermanager().Getpeer(ipaddress)
			if err != nil {
				log.Println(err)
			}
			if peer == nil {
				peer = &store.Peer{
					IP:          ipaddress,
					ID:          uuid.New().String(),
					FileSession: store.NewSession(),
					TCPConn:     conn,
				}
				err = store.Getpeermanager().Addpeer(peer)
				if err != nil {
					log.Printf("Failed to add peer to peer manager: %v", err)
				}
			} else {
				peer.TCPConn = conn
			}
			go c.handleIncomingconsent(conn, ipaddress)
		}
	}
}

func (c *Consent) handleIncomingconsent(conn net.Conn, ipaddress string) error {
	var message ConsentMessage
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&message); err != nil {
		if err == io.EOF {
			return fmt.Errorf("connection closed unexpectedly during message decoding")
		}
		return fmt.Errorf("decoding error details: \n- Error: %v \n- Error type: %T \n- Connection: %v \n- Remote Address: %v",
			err, err, conn, conn.RemoteAddr())
	}

	log.Printf("received consent request of type: %v, metadata: %v", message.Type, message.Metadata)

	switch message.Type {
	case INITIAL:
		log.Printf("Initial connection request received. Do you accept? (y/n): ")
		consent := c.getInput()
		c.sendconsent(conn, &ConsentMessage{
			Type: INITIALRES,
			Metadata: map[string]string{
				"Accepted": fmt.Sprintf("%v", consent),
			},
		})
		store.Getpeermanager().Changeconsent(ipaddress, consent)

	case FILE:
		log.Printf("File r equest recived - %v", message)
		consent := c.getInput()
		c.sendconsent(conn, &ConsentMessage{
			Type: FILERES,
			Metadata: map[string]string{
				"Accepted": fmt.Sprintf("Consent response - %v", consent),
			},
		})

	case INITIALRES, FILERES:
		consentStr, exists := message.Metadata["Accepted"]
		if !exists {
			log.Printf("Consent response from %s , Rejecting...", ipaddress)
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return fmt.Errorf("missing 'Accepted' key in metadata")
		}
		var consent bool
		if consentStr == "true" || consentStr == "True" || consentStr == "1" {
			consent = true
		} else if consentStr == "false" || consentStr == "False" || consentStr == "0" {
			consent = false
		} else {
			log.Printf("Invalid consent value '%s'. Rejecting.", consentStr)
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return fmt.Errorf("invalid consent value: %s", consentStr)
		}
		store.Getpeermanager().Changeconsent(ipaddress, consent)
		log.Printf("Consent status for peer %s updated to %v", ipaddress, consent)

		//c.notifychan <- message
	case ERROR:
		return fmt.Errorf("unknown error occured %v", message)

	default:
		return fmt.Errorf("unknown consent type received .Automatically rejecting. %v", message)
	}

	return nil
}

func (cs *Consent) getInput() bool {
	var input string
	for {
		fmt.Scanln(&input)
		if input == "y" || input == "Y" {
			return true
		} else if input == "n" || input == "N" {
			return false
		}
		fmt.Println("Invalid input. Please enter 'y' or 'n':")
	}
}

func (cs *Consent) Sendmessage(ipaddress string, msg *ConsentMessage) error {
	peeraddress := fmt.Sprintf("%s:%d", ipaddress, CONSENTPORT)
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		log.Println(err)
	}
	if peer == nil {
		//establish the connection to the peer for TCP
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			log.Printf("Failed to add peer to peer manager: %v", err)
		}
	}
	if peer.TCPConn == nil {
		conn, err := net.Dial("tcp", peeraddress)
		if err != nil {
			log.Println(err)
			return err
		}
		peer.TCPConn = conn
	}
	return cs.sendconsent(peer.TCPConn, msg)
}

var (
	consent   *Consent
	singleton sync.Once
)

func Getconsent() *Consent {
	ctx, cancel := context.WithCancel(context.Background())
	singleton.Do(func() {
		consent = &Consent{
			ctx:        ctx,
			notifychan: make(chan ConsentMessage, 100),
			cancel:     cancel,
		}
	})
	return consent
}
