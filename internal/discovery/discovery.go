package discovery

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/mdns"
)

func EmitPeerDiscovery(port int) {
	// const port = 7890
	const serviceName = "_file-transfer._tcp"
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname : %v", err)
	}
	info := []string{fmt.Sprintf("Emit peer discovery on %s", hostname)}
	service, _ := mdns.NewMDNSService(hostname, serviceName, "", "", port, nil, info)
	config := &mdns.Config{
		Zone: service,
	}
	server, _ := mdns.NewServer(config)

	defer server.Shutdown()
	fmt.Println("Peer Discovery beacon started")
}

func DiscoverPeers() []*mdns.ServiceEntry {
	const serviceName = "_file-transfer._tcp"
	peerChan := make(chan *mdns.ServiceEntry, 10)
	var peerentries []*mdns.ServiceEntry

	go func() {
		for peer := range peerChan {
			peerentries = append(peerentries, peer)
		}
	}()

	params := mdns.DefaultParams(serviceName)
	params.DisableIPv6 = true
	params.Entries = peerChan
	params.Timeout = 1 * time.Second
	params.Domain = "local."
	err := mdns.Query(params)
	if err != nil {
		log.Printf("Error while discovering peers: %v", err)
	}
	close(peerChan)
	return peerentries
}
