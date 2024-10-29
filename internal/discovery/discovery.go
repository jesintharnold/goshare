package discovery

import (
	"context"
	"log"
	"os"
	"github.com/grandcat/zeroconf"
)

func EmitPeerDiscovery(shutdown <- chan os.Signal) {
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
	<- shutdown
}

func DiscoverPeers(shutdown <- chan os.Signal) {
	const serviceName = "_filetransfer._tcp"
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			// log.Println(entry)
			log.Printf("Discovered peer: %s - %s\n", entry.HostName, entry.AddrIPv4)
		}
	}()
	
	disparentctx,disparentcancel:=context.WithCancel(context.Background())
	defer disparentcancel()
	log.Println("Alive Discovery")
	err = resolver.Browse(disparentctx, serviceName, "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}
	<- shutdown
	log.Println("Shutting down peer discovery.")
	close(entries)
}
