package main

import (
	"fmt"
	"goshare/internal/discovery"
	"log"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <port>")
	}

	// Convert port from string to integer
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Error converting port to integer: %v", err)
	}
	fmt.Println("Port:", port)

	go discovery.EmitPeerDiscovery(port)
	stop := make(chan bool)

	go func() {
		for {
			peers := discovery.DiscoverPeers()
			fmt.Printf("Discovered %d peers:\n", len(peers))
			for _, peer := range peers {
				fmt.Printf("- Host: %s, IP: %v, Port: %d\n",
					peer.Host, peer.AddrV4, peer.Port)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	<-stop
}
