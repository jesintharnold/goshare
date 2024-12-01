package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

//Listen for global consents
//Send consents to other peers
// Must have a map - to store the peer consent status
// If any update like cancel or anyhthing. we can simply destroy the peer status from here as well as from peer file transfer session
// Get methods like getvalidateduser (returns from map whether the user is validated or not)

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
	consentmap map[string]bool
	ctx        context.Context
	mu         sync.Mutex
	notifychan chan ConsentMessage
}

const CONSENTPORT = 42424

//Methods
//Listen for connections
//Send message

func (c *Consent) Init() error {
	ctx, _ := context.WithCancel(context.Background())
	c.ctx = ctx
	c.consentmap = make(map[string]bool)
	c.notifychan = make(chan ConsentMessage, 100)
	// defer cancel()
	return nil
}

func (c *Consent) Sendconsent(conn net.Conn, msg *ConsentMessage) error {
	msgencoder := json.NewEncoder(conn)
	if err := msgencoder.Encode(msg); err != nil {
		return fmt.Errorf("error while sending consent message to user \n error - %s\nremote address -%s", err, conn.RemoteAddr().String())
	}
	return nil
}

func (c *Consent) Receiveconsent() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", CONSENTPORT))
	if err != nil {
		return fmt.Errorf("failed to listen for incoming connections on port - %s", CONSENTPORT)
	}
	for {
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("recived context cancel signal, listening stopping on port - %s", CONSENTPORT)
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

			c.mu.Lock()
			c.consentmap[ipaddress] = false
			c.mu.Unlock()
			go c.handleIncomingconsent(conn, ipaddress)
		}
	}
}

func (c *Consent) handleIncomingconsent(conn net.Conn, ipaddress string) error {
	var message ConsentMessage
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&message); err != nil {
		if err == io.EOF {
			return fmt.Errorf("Connection closed unexpectedly during message decoding")
		}
		return fmt.Errorf("Decoding error details: \n- Error: %v \n- Error type: %T \n- Connection: %v \n- Remote Address: %v",
			err, err, conn, conn.RemoteAddr())
	}

	log.Printf("Received consent request of type: %s, metadata: %s", message.Type, message.Metadata)

	//Before going to swith check this should belong to same machine

	switch message.Type {
	case INITIAL:
		log.Printf("Initial connection request received. Do you accept? (y/n): ")
		consent := c.getInput()
		c.Sendconsent(conn, &ConsentMessage{
			Type: INITIALRES,
			Metadata: map[string]string{
				"Accepted": fmt.Sprintf("Consent response - %s", consent),
			},
		})

	case FILE:
		log.Printf("File request recived - %v", message)
		consent := c.getInput()
		c.Sendconsent(conn, &ConsentMessage{
			Type: FILERES,
			Metadata: map[string]string{
				"Accepted": fmt.Sprintf("Consent response - %s", consent),
			},
		})

	case INITIALRES, FILERES:
		c.notifychan <- message
	case ERROR:
		log.Printf("unknown error occured %v", message)
	default:
		log.Println("Unknown consent type received .Automatically rejecting.")
	}

	return nil
}

func (c *Consent) Checkuserconsent(peer string) bool {
	return c.consentmap[peer]
}

func (c *Consent) Closepeer(peer string) {
	c.mu.Lock()
	delete(c.consentmap, peer)
	c.mu.Unlock()
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
