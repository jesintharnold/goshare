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
)

type ConsentMessage struct {
	Type     CONSENT
	Metadata map[string]string
}

type ConsentResponse struct {
	Accepted bool
}

type Consent struct {
	Conn net.Conn
	Ctx  context.Context
}

func (cs *Consent) RequestConsent(msg *ConsentMessage) (consent bool, err error) {
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
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		if response.Accepted {
			return response.Accepted, nil
		} else {
			return true, nil
		}
	case err := <-errorChan:
		return false, err

	case <-cs.Ctx.Done():
		return false, fmt.Errorf("consent request cancelled , %v", cs.Ctx.Err())
	}
}

func (cs *Consent) HandleIncomingConsent() bool {
	decoder := json.NewDecoder(cs.Conn)
	var msg ConsentMessage
	if err := decoder.Decode(&msg); err != nil {
		log.Printf("Failed to decode consent message: %v", err)
		return false
	}
	log.Printf("Received consent request of type: %v, metadata: %v", msg.Type, msg.Metadata)
	var consentGranted bool
	switch msg.Type {
	case INITIAL_CONNECTION:
		fmt.Println("Initial connection request received. Do you accept? (y/n): ")
	case FILE_TRANSFER:
		fmt.Println("File transfer request received. Do you accept? (y/n): ")
	default:
		fmt.Println("Unknown consent type received. Automatically rejecting.")
		consentGranted = false
	}
	if msg.Type == INITIAL_CONNECTION || msg.Type == FILE_TRANSFER {
		var input string

		//Handle incoming consent from the command line
		fmt.Scanln(&input)
		consentGranted = input == "y" || input == "Y"
	}
	response := ConsentResponse{Accepted: consentGranted}
	encoder := json.NewEncoder(cs.Conn)
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Failed to send consent response: %v", err)
		return false
	}

	if consentGranted {
		log.Println("Consent granted.")
		return true
	} else {
		log.Println("Consent denied.")
		return false
	}

}

func NewConsent(conn net.Conn, ctx context.Context) *Consent {
	return &Consent{
		Conn: conn,
		Ctx:  ctx,
	}
}
