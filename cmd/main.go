package main

import (
	"bufio"
	"context"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/store"
	"goshare/internal/transfer"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func main() {
	context, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopchan := make(chan os.Signal, 1)
	signal.Notify(stopchan, os.Interrupt, syscall.SIGTERM)

	notifychan := make(chan string, 10)

	manager := store.Getpeermanager()
	if manager == nil {
		log.Fatal("Failed to initialize peer manager")
	}
	cons := consent.Getconsent()
	if cons == nil {
		log.Fatal("Failed to initialize consent manager")
	}

	//we are injecting the notify channel
	cons.SetupNotify(notifychan)

	qListener, qSender := transfer.Getfileshare()
	if qListener == nil || qSender == nil {
		log.Fatal("Failed to initialize file share components")
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		cons.Receiveconsent()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		qListener.QUICListener(context)
	}()

	wg.Add(1)
	go func(notifychan chan string) {
		defer wg.Done()
		CLI(notifychan)
	}(notifychan)

	<-stopchan
	log.Println("Shutdown signal received, cleaning up...")
	cancel()
	wg.Wait()
	os.Exit(1)
	log.Println("Shutdown complete")
}

func CLI(notifychan chan string) {
	fmt.Println("Goshare - version 0.1.0")	
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case consentpromt := <-notifychan:
			fmt.Println(consentpromt)
			fmt.Print("> ")
			promptres, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while reading input \n %v", err)
				continue
			}

			promptres = strings.TrimSpace(promptres)
			if promptres == "" {
				fmt.Fprint(os.Stderr, "No response given")
				continue
			}

			notifychan <- promptres

		default:
			fmt.Print("> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while reading input \n %v", err)
				continue
			}

			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			args := strings.Split(input, " ")
			cmd := args[0]

			switch cmd {
			case "consent":
				if len(args) != 2 {
					fmt.Println("Usage: consent <IP>")
					continue
				}
				clientip := args[1]
				// consent with ip for sending
				consent.Getconsent().Sendmessage(clientip, &consent.ConsentMessage{
					Type: consent.INITIAL,
					Metadata: map[string]string{
						"name": fmt.Sprintf("%s with %s want to Initate file share", "I am legend", "My IP Address"),
					},
				})

			case "sendfile":
				if len(args) != 3 {
					fmt.Println("Usage: sendfile <IP> <file>")
					continue
				}
				clientip := args[1]
				file_path := args[2]
				// sendfile function here
				_, fsend := transfer.Getfileshare()
				fsend.SendFile(clientip, file_path)
			case "quit":
				fmt.Println("Exiting goshare...")
				return
			case "help":
				printHelp()
			default:
				fmt.Println("Unknown command. Type 'help' for a list of commands.")
			}
		}

	}

}

func printHelp() {
	fmt.Println("Commands:")
	fmt.Println("  sendfile <IP> <file>: Send a file to a client.")
	fmt.Println("  consent <IP>: Give consent to a client.")
	fmt.Println("  quit: Exit the CLI.")
	fmt.Println("  help: Show this help message.")
}
