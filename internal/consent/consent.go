package consent

import (
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

type ConsentService struct {
	Conn net.Conn
}

func (cs *ConsentService) RequestConsent(msg *ConsentMessage) {
	encoder := json.NewEncoder(cs.Conn)
	if err := encoder.Encode(msg); err != nil {
		log.Printf("failed to send consent request: %v", err)
		return
	}

	responseChan := make(chan ConsentResponse)
	errorChan := make(chan error)

	go func() {
		var response ConsentResponse
		decoder := json.NewDecoder(cs.Conn)
		if err := decoder.Decode(&response); err != nil {
			errorChan <- err
			return
		}
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		if !response.Accepted {
			log.Println("Consent rejected by receiver.")
		} else {
			fmt.Println("Consent granted by receiver.")
		}
	case err := <-errorChan:
		log.Printf("Failed to receive consent response: %v", err)
	}
}

func (cs *ConsentService) HandleIncomingConsent() {
	decoder := json.NewDecoder(cs.Conn)
	var msg ConsentMessage
	if err := decoder.Decode(&msg); err != nil {
		log.Printf("Failed to decode consent message: %v", err)
		return
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
		fmt.Scanln(&input)
		consentGranted = input == "y" || input == "Y"
	}
	response := ConsentResponse{Accepted: consentGranted}
	encoder := json.NewEncoder(cs.Conn)
	if err := encoder.Encode(&response); err != nil {
		log.Printf("Failed to send consent response: %v", err)
		return
	}

	if consentGranted {
		log.Println("Consent granted.")
	} else {
		log.Println("Consent denied.")
	}
}
