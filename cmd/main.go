package main

import (
	"context"
	"goshare/internal/discovery"
	"goshare/internal/transfer"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	// fileshare.GenerateParentCerts()

	// time.Sleep(30*time.Second)

	// fileshare.GenerateClientCerts()

	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
	role := strings.ToUpper(os.Args[1])
	switch role {
	case "E":
		log.Println("Starting in Emitter mode...")
		go discovery.EmitPeerDiscovery(ctx)

		pm := &transfer.PeerManager{}
		go pm.ListenToPeer()

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
		test_transfer.ConnectToPeer("jesinth-sender", "craxy-jesi", "192.168.0.105", 42424)

	default:
		log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
	}
	<-stopchan
	cancel()
	log.Println("Shutting down goshare application.")
}
