package main

import (
	"context"
	"goshare/internal/fileshare"
	"log"
	"os"
	"strings"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}
	role := strings.ToUpper(os.Args[1])

	//---
	const QUIC_PORT = 42425

	//--
	switch role {
	case "E":
		ctx, _ := context.WithCancel(context.Background())
		pm := fileshare.NewFileshare(ctx)
		go pm.ListenPeer("192.168.0.105", ctx)

	case "D":
		ctx, _ := context.WithCancel(context.Background())
		pm := fileshare.NewFileshare(ctx)
		pm.ConnectPeer("192.168.0.102")
	}

}
