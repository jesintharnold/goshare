package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"goshare/internal/discovery"
	"goshare/internal/transfer"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/quic-go/quic-go"
)

func main() {

	// fileshare.GenerateParentCerts()

	// time.Sleep(30*time.Second)

	// fileshare.GenerateClientCerts()

	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}


	//---
	const QUIC_PORT = 42425

    // Load certificates
    certificate, err := tls.LoadX509KeyPair(filepath.Join("CERT", "client.crt"), filepath.Join("CERT", "client.key"))
    if err != nil {
        log.Fatalf("Error loading certificates: %v", err)
    }

    tlsConfig := &tls.Config{
        Certificates:       []tls.Certificate{certificate},
        InsecureSkipVerify: true,
    }

	//--

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
	role := strings.ToUpper(os.Args[1])
	switch role {
	case "E":
		log.Println("Starting in Emitter mode...")
		go discovery.EmitPeerDiscovery(ctx)

		// pm := transfer.NewPeerManager()
		// go pm.ListenToPeer()

		listener, err := quic.ListenAddr(fmt.Sprintf(":%d", QUIC_PORT), tlsConfig, nil)
		if err != nil {
			log.Fatalf("Failed to start QUIC listener: %v", err)
		}
		defer listener.Close()
		log.Printf("QUIC Listening on port %d", QUIC_PORT)

		conn, err := listener.Accept(ctx)
		if err != nil {
			log.Printf("Failed to accept: %v", err)
			return
		}
		log.Printf("Accepted connection from: %v", conn.RemoteAddr())

	case "D":
		log.Println("Starting in Discoverer mode...")
		go discovery.DiscoverPeers(ctx)
		// peerInfo := discovery.PeerInfo{
		// 	ID:        "jesi",
		// 	Name:      "craxy-jesi",
		// 	IPAddress: "192.168.0.105",
		// 	Port:      42424,
		// }

		test_transfer := &transfer.PeerConnection{}
		test_transfer.ConnectToPeer("jesinth-sender", "craxy-jesi", "192.168.0.102", 42424)

	default:
		log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
	}
	<-stopchan
	cancel()
	log.Println("Shutting down goshare application.")
}
