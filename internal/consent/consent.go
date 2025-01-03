package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"goshare/internal/store"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

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
	ctx          context.Context
	cancel       context.CancelFunc
	notifychan   chan ConsentNotify
	responsechan chan ConsentResponse
}

const CONSENTPORT = 42424

type ConsentResponse struct {
	IP     string
	Status string
}

type ConsentNotify struct {
	IP      string
	Message string
}

func (c *Consent) SetupNotify(notify chan ConsentNotify, response chan ConsentResponse) {
	c.notifychan = notify
	c.responsechan = response
}

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
					continue
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
		fmt.Printf("Consent request from %s. Accept? (y/n):", conn.RemoteAddr().String())
		c.notifychan <- ConsentNotify{
			IP:      ipaddress,
			Message: fmt.Sprintf("Consent request from %s. Accept? (y/n):", conn.RemoteAddr().String()),
		}
		log.Println("Notification sent to channel")

		select {
		case response := <-c.responsechan:
			res_status := strings.ToLower(response.Status)
			res_ip := response.IP
			log.Println("I am a bitch")
			var consent bool
			if res_status == "y" || res_status == "yes" {
				fmt.Printf("Response given - %s\n", res_status)
				consent = true
			} else if res_status == "n" || res_status == "no" {
				fmt.Printf("Response given - %s\n", res_status)
				consent = false
			}

			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", res_ip, CONSENTPORT))
			if err != nil {
				log.Println(err)
				return err
			}
			fmt.Println("Sending the consent we see if this is working")
			c.sendconsent(conn, &ConsentMessage{
				Type: INITIALRES,
				Metadata: map[string]string{
					"Accepted": fmt.Sprintf("%v", consent),
				},
			})
			store.Getpeermanager().Changeconsent(res_ip, consent)

		case <-time.After(30 * time.Second): // Timeout after 30 seconds
			log.Println("No response received in time.")
			return fmt.Errorf("timeout waiting for response from %s", conn.RemoteAddr().String())
		}

	case FILE:
		log.Printf("File request recived - %v", message)
		c.notifychan <- ConsentNotify{
			IP:      ipaddress,
			Message: fmt.Sprintf("Consent request from %s. Accept? (y/n):", conn.RemoteAddr().String()),
		}

		select {
		case response := <-c.responsechan:
			res_status := strings.ToLower(response.Status)
			_ = response.IP
			var consent bool
			if res_status == "y" || res_status == "yes" {
				fmt.Printf("Response given - %s\n", res_status)
				consent = true
			} else if res_status == "n" || res_status == "no" {
				fmt.Printf("Response given - %s\n", res_status)
				consent = false
			}

			c.sendconsent(conn, &ConsentMessage{
				Type: FILERES,
				Metadata: map[string]string{
					"Accepted": fmt.Sprintf("Consent response - %v", consent),
				},
			})

		case <-time.After(30 * time.Second):
			log.Println("No response received in time.")
			return fmt.Errorf("timeout waiting for response from %s", conn.RemoteAddr().String())
		}

	case INITIALRES, FILERES:
		fmt.Printf("Initial response  - %v", message)
		consentStr, exists := message.Metadata["Accepted"]
		fmt.Printf("consent status we requested bitch - %v", consentStr)
		if !exists {
			log.Printf("Consent response from %s , Rejecting...", ipaddress)
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return fmt.Errorf("missing 'Accepted' key in metadata")
		}
		var consent bool
		consentStr = strings.ToLower(consentStr)
		if consentStr == "true" || consentStr == "1" {
			consent = true
		} else if consentStr == "false" || consentStr == "0" {
			consent = false
		} else {
			log.Printf("Invalid consent value '%s'. Rejecting.", consentStr)
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return fmt.Errorf("invalid consent value: %s", consentStr)
		}
		store.Getpeermanager().Changeconsent(ipaddress, consent)
		fmt.Fprintf(os.Stdout, "Consent status for peer %s updated to %v", ipaddress, consent)

	case ERROR:
		return fmt.Errorf("unknown error occured %v", message)

	default:
		return fmt.Errorf("unknown consent type received .Automatically rejecting. %v", message)
	}

	return nil
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
			ctx:    ctx,
			cancel: cancel,
		}
	})
	return consent
}
