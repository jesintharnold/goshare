package main

import (
	"context"
	"goshare/internal/transfer"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	// if len(os.Args) < 2 {
	// 	log.Fatal("Usage: program [E/D] (E for Emitter, D for Discoverer)")
	// }
	// role := strings.ToUpper(os.Args[1])
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)
	// switch role {
	// case "R":
	// 	reciver := transfer.NewPeerManager()
	// 	reciver.ListenToPeer()
	// 	// pm := fileshare.NewFileshare(ctx)
	// 	// pm.ListenPeer("192.168.0.105", ctx)
	// 	<-stopchan
	// case "S":
	// 	log.Println("Sender mode started")
	// 	peercon := transfer.NewPeerConnection("Test-1", "Jesinth-sender", "192.168.0.103", nil)
	// 	peercon.ConnectToPeer()
	// 	<-stopchan
	// }

	// consent.Getconsent().Receiveconsent()
	listen, _ := transfer.Getfileshare()
	listen.QUICListener(context.Background())
	<-stopchan
}
