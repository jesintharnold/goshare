package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"goshare/internal/errors"
	"goshare/internal/store"
	"io"
	"net"
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
		return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Error while sending consent message to user : %s", conn.RemoteAddr().String()), err)
	}
	return nil
}

func (c *Consent) Receiveconsent() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", CONSENTPORT))
	errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Started Listening for consents on port - %d", CONSENTPORT), err)
	if err != nil {
		return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("failed to listen for incoming connections on port - %d", CONSENTPORT), err)
	}
	for {
		select {
		case <-c.ctx.Done():
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("recived context cancel signal, listening stopping on port - %d", CONSENTPORT), nil)
		default:
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			ipaddress, _, err := net.SplitHostPort(conn.RemoteAddr().String())
			if err != nil {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", "Error while extracting IP address for quic connection", err)
			}
			peer, err := store.Getpeermanager().Getpeer(ipaddress)
			if err != nil {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("%v", err.Error()), err)
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
					errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", err.Error(), err)
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
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", "connection closed unexpectedly during message decoding", nil)
		}
		return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("connection closed  - %s unexpectedly during message decoding - %s", conn.RemoteAddr().String(), err.Error()), err)
	}

	switch message.Type {
	case INITIAL:
		c.notifychan <- ConsentNotify{
			IP:      ipaddress,
			Message: fmt.Sprintf("Consent request from %s. Accept? (y/n) ", conn.RemoteAddr().String()),
		}

		select {
		case response := <-c.responsechan:
			res_status := strings.ToLower(response.Status)
			res_ip := response.IP
			var consent bool
			if res_status == "y" || res_status == "yes" {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Response given - %s", "Yes"), nil)
				consent = true
			} else if res_status == "n" || res_status == "no" {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Response given - %s", "No"), nil)
				consent = false
			}

			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", res_ip, CONSENTPORT))
			if err != nil {
				return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Error while connecting to the peer - %s", res_ip), err)
			}
			c.sendconsent(conn, &ConsentMessage{
				Type: INITIALRES,
				Metadata: map[string]string{
					"Accepted": fmt.Sprintf("%v", consent),
				},
			})
			store.Getpeermanager().Changeconsent(res_ip, consent)
			conn.Close()

		case <-time.After(30 * time.Second): // Timeout after 30 seconds
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("timeout waiting for response from %s", conn.RemoteAddr().String()), nil)
		}

	case FILE:
		c.notifychan <- ConsentNotify{
			IP:      ipaddress,
			Message: fmt.Sprintf("Consent request - %s \n Accept? (y/n):", conn.RemoteAddr().String()),
		}

		select {
		case response := <-c.responsechan:
			res_status := strings.ToLower(response.Status)
			_ = response.IP
			var consent bool
			if res_status == "y" || res_status == "yes" {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Response given - %s", "Yes"), nil)
				consent = true
			} else if res_status == "n" || res_status == "no" {
				errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Response given - %s", "No"), nil)
				consent = false
			}

			c.sendconsent(conn, &ConsentMessage{
				Type: FILERES,
				Metadata: map[string]string{
					"Accepted": fmt.Sprintf("Consent response - %v", consent),
				},
			})

		case <-time.After(30 * time.Second):
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("timeout waiting for response from %s", conn.RemoteAddr().String()), nil)
		}

	case INITIALRES, FILERES:
		consentStr, exists := message.Metadata["Accepted"]
		if !exists {
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("missing 'Accepted' key in metadata in peer - %s", ipaddress), nil)
		}
		var consent bool
		consentStr = strings.ToLower(consentStr)
		if consentStr == "true" || consentStr == "1" {
			consent = true
		} else if consentStr == "false" || consentStr == "0" {
			consent = false
		} else {
			store.Getpeermanager().Changeconsent(ipaddress, false)
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Invalid consent value: %s", consentStr), nil)
		}
		store.Getpeermanager().Changeconsent(ipaddress, consent)
		errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Consent status for peer %s updated to %v", ipaddress, consent), nil)

	case ERROR:
		return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("unknown error occured %v", message), nil)
	default:
		return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("unknown consent type received .Automatically rejecting. %v", message), nil)
	}

	return nil
}

func (cs *Consent) Sendmessage(ipaddress string, msg *ConsentMessage) error {
	peeraddress := fmt.Sprintf("%s:%d", ipaddress, CONSENTPORT)
	peer, err := store.Getpeermanager().Getpeer(ipaddress)
	if err != nil {
		errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", err.Error(), err)
	}
	if peer == nil {
		peer = &store.Peer{
			IP:          ipaddress,
			ID:          uuid.New().String(),
			FileSession: store.NewSession(),
		}
		err = store.Getpeermanager().Addpeer(peer)
		if err != nil {
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", err.Error(), err)
		}
	}
	if peer.TCPConn == nil {
		conn, err := net.Dial("tcp", peeraddress)
		if err != nil {
			return errors.NewError(errors.ErrConsent, errors.ERROR, "CONSENT", fmt.Sprintf("Error while connecting to the peer - %s", peeraddress), err)
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
