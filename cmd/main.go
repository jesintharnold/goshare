package main

import (
	"goshare/internal/transfer"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	}
	role := strings.ToUpper(os.Args[1])
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
	switch role {
	case "R":
		reciver := transfer.NewPeerManager()
		reciver.ListenToPeer()
		// pm := fileshare.NewFileshare(ctx)
		// pm.ListenPeer("192.168.0.105", ctx)
		<-stopchan
	case "S":
		log.Println("Sender mode started")
		transfer.NewPeerConnection("Test-1", "Jesinth-sender", "192.168.0.105", nil)

		<-stopchan
	}

}
