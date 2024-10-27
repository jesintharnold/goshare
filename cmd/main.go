package main

import (
	"goshare/internal/discovery"
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
    switch role {
    case "E":
        log.Println("Starting in Emitter mode...")
        go discovery.EmitPeerDiscovery()
    case "D":
        log.Println("Starting in Discoverer mode...")
        go discovery.DiscoverPeers()
    default:
        log.Fatal("Invalid role. Use 'E' for Emitter or 'D' for Discoverer")
    }

	stopchan:=make(chan os.Signal,1)
	signal.Notify(stopchan,os.Interrupt,syscall.SIGTERM)
    <-stopchan
    log.Println("Shutting Down")
}