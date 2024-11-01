package main

import (
	"goshare/internal/discovery"
	"goshare/internal/transfer"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
	role := strings.ToUpper(os.Args[1])
	switch role {
	case "E":
		log.Println("Starting in Emitter mode...")
		go discovery.EmitPeerDiscovery(stopchan)
		transferService := transfer.TransferService{}
		go transferService.ListenToPeer()
	case "D":
		log.Println("Starting in Discoverer mode...")
		go discovery.DiscoverPeers(stopchan)
		time.Sleep(1 * time.Minute)
		peerInfo := discovery.PeerInfo{
			ID:        "jesi",
			Name:      "craxy-jesi",
			IPAddress: "192.168.0.105",
			Port:      42424,
		}

		test_transfer := transfer.TransferService{
			Peerinfo: peerInfo,
		}
		test_transfer.ConnectToPeer()
	default:
		log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
	}
	<-stopchan
	log.Println("Shutting down goshare application.")
}
