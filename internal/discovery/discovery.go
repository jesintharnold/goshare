package discovery

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/grandcat/zeroconf"
)

func EmitPeerDiscovery(ctx context.Context) {
	const port = 42424
	const serviceName = "_filetransfer._tcp"
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname : %v", err)
	}
	//info := []string{fmt.Sprintf("Emit peer discovery on %s", hostname)}
	server, err := zeroconf.Register(hostname, serviceName, "local.", port, []string{"txtv=0", "lo=1", "la=2"}, nil)
	log.Println("Peer Discovery beacon started")
	if err != nil {
		panic(err)
	}
	defer server.Shutdown()
	<-ctx.Done()
	log.Println("Shutting down beacon")
}

func DiscoverPeers(ctx context.Context) {
	const serviceName = "_filetransfer._tcp"
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			peer := PeerInfo{
				ID:   extractID(entry.Instance),
				Name: entry.Instance,
				Port: entry.Port,
			}
			if len(entry.AddrIPv4) > 0 {
				peer.IPAddress = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				peer.IPAddress = entry.AddrIPv6[0].String()
			}

			if peer.IPAddress != "" && peer.ID != "" {
				log.Printf("Discovered peer: %v", peer)
			}
		}
	}()

	discoverCtx, discoverCancel := context.WithCancel(ctx)
	defer discoverCancel()
	err = resolver.Browse(discoverCtx, serviceName, "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}
	<-ctx.Done()
	log.Println("Shutting down peer discovery.")
}

func extractID(instance string) string {
	parts := strings.Split(instance, "-")
	if len(parts) >= 2 {
		// Get the last part which should contain the ID
		idPart := parts[len(parts)-1]
		// Remove any service suffix
		return strings.Split(idPart, ".")[0]
	}
	return ""
}
