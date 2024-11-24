package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/quic-go/quic-go"
)

// func main() {

// 	// fileshare.GenerateParentCerts()

// 	// time.Sleep(30*time.Second)

// 	// fileshare.GenerateClientCerts()

// 	if len(os.Args) < 2 {
// 		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
// 	}

// 	//---
// 	const QUIC_PORT = 42425

// 	// Load certificates
// 	certificate, err := tls.LoadX509KeyPair(filepath.Join("PARENT", "parentCA.crt"), filepath.Join("PARENT", "parent.key"))
// 	if err != nil {
// 		log.Fatalf("Error loading certificates: %v", err)
// 	}

// 	tlsConfig := &tls.Config{
// 		Certificates:       []tls.Certificate{certificate},
// 		InsecureSkipVerify: true,
// 	}

// 	//--

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
// 	stopchan := make(chan os.Signal, 1)
// 	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
// 	role := strings.ToUpper(os.Args[1])
// 	switch role {
// 	case "E":
// 		log.Println("Starting in Emitter mode...")
// 		go discovery.EmitPeerDiscovery(ctx)

// 		pm := transfer.NewPeerManager()
// 		go pm.ListenToPeer()

// 		listener, err := quic.ListenAddr(fmt.Sprintf(":%d", QUIC_PORT), tlsConfig, nil)
// 		if err != nil {
// 			log.Fatalf("Failed to start QUIC listener: %v", err)
// 		}
// 		defer listener.Close()
// 		log.Printf("QUIC Listening on port %d", QUIC_PORT)

// 		conn, err := listener.Accept(ctx)
// 		if err != nil {
// 			log.Printf("Failed to accept: %v", err)
// 			return
// 		}
// 		log.Printf("Accepted connection from: %v", conn.RemoteAddr())

// 	case "D":
// 		log.Println("Starting in Discoverer mode...")
// 		go discovery.DiscoverPeers(ctx)
// 		// peerInfo := discovery.PeerInfo{
// 		// 	ID:        "jesi",
// 		// 	Name:      "craxy-jesi",
// 		// 	IPAddress: "192.168.0.105",
// 		// 	Port:      42424,
// 		// }

// 		test_transfer := &transfer.PeerConnection{}
// 		test_transfer.ConnectToPeer("jesinth-sender", "craxy-jesi", "192.168.0.102", 42424)

// 	default:
// 		log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
// 	}
// 	<-stopchan
// 	cancel()
// 	log.Println("Shutting down goshare application.")
// }

func handleConnection(conn quic.Connection) {
	// Accept stream
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		fmt.Printf("Failed to accept stream: %v\n", err)
		return
	}
	defer stream.Close()

	// Read message
	buffer := make([]byte, 1024)
	n, err := stream.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read from stream: %v\n", err)
		return
	}

	fmt.Printf("Received message: %s\n", buffer[:n])

	// Send response
	response := "Message received successfully!"
	_, err = stream.Write([]byte(response))
	if err != nil {
		fmt.Printf("Failed to write response: %v\n", err)
		return
	}
}

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}
	role := strings.ToUpper(os.Args[1])

	//---
	const QUIC_PORT = 42425

	// Load certificates
	certificate, err := tls.LoadX509KeyPair(filepath.Join("PARENT", "parentCA.crt"), filepath.Join("PARENT", "parent.key"))
	if err != nil {
		log.Fatalf("Error loading certificates: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true,
	}

	//--
	switch role {
	case "E":
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		listener, err := quic.ListenAddr(":42425", tlsConfig, nil)
		if err != nil {
			fmt.Printf("Failed to start server: %v\n", err)
			os.Exit(1)
		}
		defer listener.Close()

		fmt.Println("Server listening on :42425")

		for {
			conn, err := listener.Accept(context.Background())
			if err != nil {
				fmt.Printf("Failed to accept connection: %v\n", err)
				continue
			}

			fmt.Printf("New connection from: %s\n", conn.RemoteAddr())

			// Handle the connection
			go handleConnection(conn)
		}

	case "D":
		serverAddr := "192.168.0.102:42425"
		tlsConf := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"quic-test"},
		}

		conn, err := quic.DialAddr(context.Background(), serverAddr, tlsConf, nil)
		if err != nil {
			fmt.Printf("Failed to connect to server: %v\n", err)
			os.Exit(1)
		}
		defer conn.CloseWithError(0, "")

		// Create a stream
		stream, err := conn.OpenStreamSync(context.Background())
		if err != nil {
			fmt.Printf("Failed to open stream: %v\n", err)
			os.Exit(1)
		}
		defer stream.Close()

		// Send test message
		message := "Hello from client!"
		_, err = stream.Write([]byte(message))
		if err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
			os.Exit(1)
		}

		// Read response
		buffer := make([]byte, 1024)
		n, err := stream.Read(buffer)
		if err != nil {
			fmt.Printf("Failed to read response: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Server response: %s\n", buffer[:n])
	}

}
