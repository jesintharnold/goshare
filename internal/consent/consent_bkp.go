// package consent

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net"
// )

// type CONSENT int

// const (
// 	INITIAL_CONNECTION CONSENT = iota
// 	FILE_TRANSFER
// 	READINESS_NOTIFICATION
// 	ERROR_CONSENT
// )

// type ConsentMessage struct {
// 	Type     CONSENT
// 	Metadata map[string]string
// }

// type ConsentResponse struct {
// 	Accepted bool
// 	Type     CONSENT
// }

// type Consent struct {
// 	Conn net.Conn
// 	Ctx  context.Context
// }

// func (cs *Consent) RequestConsent(msg *ConsentMessage) (bool, error) {
// 	encoder := json.NewEncoder(cs.Conn)
// 	if err := encoder.Encode(msg); err != nil {
// 		return false, fmt.Errorf("failed to send consent request: %v", err)
// 	}

// 	responseChan := make(chan ConsentResponse)
// 	errorChan := make(chan error)

// 	go func() {
// 		var response ConsentResponse
// 		decoder := json.NewDecoder(cs.Conn)
// 		if err := decoder.Decode(&response); err != nil {
// 			errorChan <- fmt.Errorf("failed to decode consent response: %w", err)
// 			return
// 		}
// 		log.Printf("Consent response - %v", response)
// 		responseChan <- response
// 	}()

// 	select {
// 	case response := <-responseChan:
// 		if response.Accepted {
// 			return response.Accepted, nil
// 		} else {
// 			return false, fmt.Errorf("consent request rejected")
// 		}
// 	case err := <-errorChan:
// 		return false, err
// 	case <-cs.Ctx.Done():
// 		return false, fmt.Errorf("consent request cancelled , %v", cs.Ctx.Err())
// 	}
// }

// func (cs *Consent) getInput() bool {
// 	var input string
// 	//Handle incoming consent from the command line
// 	fmt.Scanln(&input)
// 	return input == "y" || input == "Y"
// }

// func (cs *Consent) HandleIncomingConsent() (ConsentMessage, bool) { 
// 	log.Println("Looking for Incoming consents bitch")

// 	if cs.Conn == nil {
// 		log.Println("Connection is nil before decoding")
// 		return ConsentMessage{}, false
// 	}

// 	// if tcpConn, ok := cs.Conn.(*net.TCPConn); ok {
// 	// 	file, err := tcpConn.File()
// 	// 	if err != nil || file == nil {
// 	// 		log.Println("Connection appears to be closed")
// 	// 		return ConsentMessage{}, false
// 	// 	}
// 	// }

// 	decoder := json.NewDecoder(cs.Conn)
// 	var msg ConsentMessage
// 	if err := decoder.Decode(&msg); err != nil {
// 		log.Printf("Decoding error details: \n- Error: %v \n- Error type: %T \n- Connection: %v \n- Remote Address: %v",
// 			err, err, cs.Conn, cs.Conn.RemoteAddr())
// 		if err == io.EOF {
// 			log.Println("Connection closed unexpectedly during message decoding")
// 		}
// 		return msg, false
// 	}
// 	log.Printf("Received consent request of type: %v, metadata: %v", msg.Type, msg.Metadata)
// 	var consentGranted bool
// 	var response ConsentResponse

// 	log.Printf("Consent message - %v", msg)

// 	switch msg.Type {
// 	case INITIAL_CONNECTION:
// 		fmt.Println("Initial connection request received. Do you accept? (y/n): ")
// 		consentGranted = cs.getInput()
// 		response = ConsentResponse{Accepted: consentGranted, Type: INITIAL_CONNECTION}
// 	case FILE_TRANSFER:
// 		fmt.Println("File transfer request received. Do you accept? (y/n): ")
// 		consentGranted = cs.getInput()
// 		response = ConsentResponse{Accepted: consentGranted, Type: FILE_TRANSFER}
// 	case READINESS_NOTIFICATION:
// 		response = ConsentResponse{Accepted: true, Type: READINESS_NOTIFICATION}
// 		fmt.Printf("Readiness notification recived - %s", cs.Conn.RemoteAddr())
// 		return msg, true
// 	default:
// 		fmt.Println("Unknown consent type received .Automatically rejecting.")
// 		response = ConsentResponse{Accepted: false, Type: ERROR_CONSENT}
// 	}
// 	//Reset the consent message

// 	encoder := json.NewEncoder(cs.Conn)
// 	if err := encoder.Encode(&response); err != nil {
// 		log.Printf("Failed to send consent response: %v", err)
// 		return msg, false
// 	}

// 	if consentGranted {
// 		log.Println("Consent granted.")
// 		return msg, true
// 	} else {
// 		log.Println("Consent denied.")
// 		return msg, false
// 	}

// }

// func (cs *Consent) NotifyReadiness() error {
// 	readinessMsg := ConsentMessage{
// 		Type: READINESS_NOTIFICATION,
// 		Metadata: map[string]string{
// 			"msg": "Peer is ready to accept QUIC connections",
// 		},
// 	}

// 	resEncoder := json.NewEncoder(cs.Conn)
// 	if err := resEncoder.Encode(&readinessMsg); err != nil {
// 		return fmt.Errorf("%s - %v", cs.Conn.RemoteAddr(), err)
// 	}
// 	fmt.Printf("%s peer QUIC readiness notification - sent", cs.Conn.RemoteAddr())
// 	return nil
// }

// func (cs *Consent) Close() error {
// 	return cs.Conn.Close()
// }

// func NewConsent(conn net.Conn, ctx context.Context) *Consent {
// 	return &Consent{
// 		Conn: conn,
// 		Ctx:  ctx,
// 	}
// }
