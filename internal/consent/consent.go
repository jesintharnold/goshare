package consent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

type CONSENT int

const (
	INITIAL_CONNECTION CONSENT = iota
	FILE_TRANSFER
	READINESS_NOTIFICATION
	ERROR_CONSENT
)

type ConsentMessage struct {
	Type     CONSENT
	Metadata map[string]string
}

type ConsentResponse struct {
	Accepted bool
	Type     CONSENT
}

type Consent struct {
	Conn net.Conn
	Ctx  context.Context
}

func (cs *Consent) RequestConsent(msg *ConsentMessage) (bool, error) {
	encoder := json.NewEncoder(cs.Conn)
	if err := encoder.Encode(msg); err != nil {
		return false, fmt.Errorf("failed to send consent request: %v", err)
	}

	responseChan := make(chan ConsentResponse)
	errorChan := make(chan error)

	go func() {
		var response ConsentResponse
		decoder := json.NewDecoder(cs.Conn)
		if err := decoder.Decode(&response); err != nil {
			errorChan <- fmt.Errorf("failed to decode consent response: %w", err)
			return
		}
		log.Printf("Consent response - %v", decoder)
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		if response.Accepted {
			return response.Accepted, nil
		}
	case err := <-errorChan:
		return false, err
	case <-cs.Ctx.Done():
		return false, fmt.Errorf("consent request cancelled , %v", cs.Ctx.Err())
	}

	return false, nil
}

func (cs *Consent) getInput() bool {
	var input string
	//Handle incoming consent from the command line
	fmt.Scanln(&input)
	return input == "y" || input == "Y"
}

func (cs *Consent) HandleIncomingConsent() (ConsentMessage, bool) {
	decoder := json.NewDecoder(cs.Conn)
	var msg ConsentMessage
	if err := decoder.Decode(&msg); err != nil {
		log.Printf("Failed to decode consent message: %v", err)
		return msg, false
	}
	log.Printf("Received consent request of type: %v, metadata: %v", msg.Type, msg.Metadata)
	var consentGranted bool
	var response ConsentResponse

	log.Printf("Consent message - %v", msg)

	switch msg.Type {
	case INITIAL_CONNECTION:
		fmt.Println("Initial connection request received. Do you accept? (y/n): ")
		consentGranted = cs.getInput()
		response = ConsentResponse{Accepted: consentGranted, Type: INITIAL_CONNECTION}
	case FILE_TRANSFER:
		fmt.Println("File transfer request received. Do you accept? (y/n): ")
		consentGranted = cs.getInput()
		response = ConsentResponse{Accepted: consentGranted, Type: FILE_TRANSFER}
	case READINESS_NOTIFICATION:
		response = ConsentResponse{Accepted: consentGranted, Type: READINESS_NOTIFICATION}
		fmt.Printf("Readiness notification recived - %s", cs.Conn.RemoteAddr())
		return msg, true
	default:
		fmt.Println("Unknown consent type received .Automatically rejecting.")
		return msg, false
	}
	encoder := json.NewEncoder(cs.Conn)
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Failed to send consent response: %v", err)
		return msg, false
	}

	if consentGranted {
		log.Println("Consent granted.")
		return msg, true
	} else {
		log.Println("Consent denied.")
		return msg, false
	}

}

func (cs *Consent) NotifyReadiness() error {
	log.Println("Sent readiness signal , from here")
	readinessMsg := ConsentMessage{
		Type: READINESS_NOTIFICATION,
		Metadata: map[string]string{
			"msg": "Peer is ready to accept QUIC connections",
		},
	}

	resEncoder := json.NewEncoder(cs.Conn)
	if err := resEncoder.Encode(&readinessMsg); err != nil {
		return fmt.Errorf("%s - %v", cs.Conn.RemoteAddr(), err)
	}
	fmt.Printf("%s peer QUIC readiness notification - sent", cs.Conn.RemoteAddr())
	return nil
}

func (cs *Consent) Close() error {
	return cs.Conn.Close()
}

func NewConsent(conn net.Conn, ctx context.Context) *Consent {
	return &Consent{
		Conn: conn,
		Ctx:  ctx,
	}
}
