package discovery

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
)

func EmitPeerDiscovery() {
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
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		// Exit by user
	case <-time.After(time.Second * 120):
		// Exit by timeout
	}
}

func DiscoverPeers() {
	const serviceName = "_filetransfer._tcp"
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			log.Println(entry)
		}
		log.Println("No more entries.")
	}(entries)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()
	err = resolver.Browse(ctx, serviceName, "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
}
