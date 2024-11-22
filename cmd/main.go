package main

import "goshare/internal/fileshare"

// import (
// 	"context"
// 	"goshare/internal/discovery"
// 	"goshare/internal/transfer"
// 	"log"
// 	"os"
// 	"os/signal"
// 	"strings"
// 	"syscall"
// )

// func main() {
// 	if len(os.Args) < 2 {
// 		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
// 	}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
// 	stopchan := make(chan os.Signal, 1)
// 	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
// 	role := strings.ToUpper(os.Args[1])
// 	switch role {
// 	case "E":
// 		log.Println("Starting in Emitter mode...")
// 		go discovery.EmitPeerDiscovery(ctx)
// 		transferService := transfer.TransferService{
// 			Ctx: ctx,
// 		}
// 		go transferService.ListenToPeer()

// 	case "D":
// 		log.Println("Starting in Discoverer mode...")
// 		go discovery.DiscoverPeers(ctx)
// 		peerInfo := discovery.PeerInfo{
// 			ID:        "jesi",
// 			Name:      "craxy-jesi",
// 			IPAddress: "192.168.0.105",
// 			Port:      42424,
// 		}

// 		test_transfer := transfer.TransferService{
// 			Peerinfo: peerInfo,
// 			Ctx:      ctx,
// 		}
// 		test_transfer.ConnectToPeer()

// 	default:
// 		log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
// 	}
// 	<-stopchan
// 	cancel()
// 	log.Println("Shutting down goshare application.")
// }

// package main

// import (
// 	"fmt"
// 	"os"
// 	"strings"
// )

// func main() {
// 	fmt.Print("Type something: ")
// 	var input = make([]byte, 100) // Allocate buffer
// 	var storage = make([]string, 0)

// 	for {
// 		n, err := os.Stdin.Read(input)
// 		if err != nil {
// 			fmt.Println("Error reading input:", err)
// 			return
// 		}

// 		words := strings.Split(strings.TrimSpace(string(input[:n])), " ")

// 		fmt.Println("You typed:", string(input[:n]))

// 		if strings.TrimSpace(string(input[:n])) == "exit" {
// 			break
// 		} else {
// 			storage = append(storage, words...)
// 		}

// 	}

// 	for index, word := range storage {
// 		fmt.Printf("Word: %s, Index: %d\n", word, index)
// 	}
// }

func main() {
	//fileshare.GenerateClientCerts()
	//fileshare.GenerateParentCerts()
}
