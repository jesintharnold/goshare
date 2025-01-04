package cmd

import (
	"bufio"
	"context"
	"fmt"
	"goshare/internal/consent"
	"goshare/internal/store"
	"goshare/internal/transfer"
	"os"
	"strings"
	"sync"
)

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  consent <IP>               - Request consent to share files with the specified IP.")
	fmt.Println("  sendfile <IP> <file>       - Send a file to the specified IP.")
	fmt.Println("  quit                       - Exit the program.")
	fmt.Println("  help                       - Show this help message.")
}

func handleConsentPrompt(reader *bufio.Reader, consentpromt consent.ConsentNotify, responsechan chan consent.ConsentResponse) {
	fmt.Print("\r\033[K")
	fmt.Println(consentpromt.Message)
	promptres, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading input: %v\n", err)
		return
	}

	promptres = strings.TrimSpace(promptres)
	if promptres == "" {
		fmt.Fprintln(os.Stderr, "No response given")
		return
	}
	responsechan <- consent.ConsentResponse{
		IP:     consentpromt.IP,
		Status: promptres,
	}
}

func processCommand(input string) {
	args := strings.Fields(input)
	cmd := args[0]

	switch cmd {
	case "consent":
		if len(args) != 2 {
			fmt.Println("Usage: consent <IP>")
			return
		}
		clientip := args[1]
		consent.Getconsent().Sendmessage(clientip, &consent.ConsentMessage{
			Type: consent.INITIAL,
			Metadata: map[string]string{
				"name": fmt.Sprintf("Requesting file share with IP: %s", clientip),
			},
		})

	case "sendfile":
		if len(args) != 3 {
			fmt.Println("Usage: sendfile <IP> <file>")
			return
		}
		clientip := args[1]
		filePath := args[2]
		_, fsend := transfer.Getfileshare()
		fsend.SendFile(clientip, filePath)

	case "quit":
		fmt.Println("Exiting Goshare...")
		os.Exit(0)

	case "help":
		printHelp()

	default:
		fmt.Println("Unknown command. Type 'help' for a list of commands.")
	}
}

func CLI(notify chan consent.ConsentNotify, responsechan chan consent.ConsentResponse) {
	fmt.Println("Goshare - version 0.1.0")
	fmt.Println("Goshare is a P2P file-sharing tool")
	fmt.Print("\n")

	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case consentpromt := <-notify:
			go handleConsentPrompt(reader, consentpromt, responsechan)
		// case errormsg := <-errors.Errorchan:
		// 	fmt.Print("\r\033[K")
		// 	fmt.Fprintln(os.Stdout, errormsg.Message)
		default:
			fmt.Print("goshare > ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while reading input: %v\n", err)
				continue
			}

			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}

			processCommand(input)
		}
	}
}

// Error channel for seding files

func Prerun(con context.Context) {
	ctx, cancel := context.WithCancel(con)
	defer cancel()
	notifychan := make(chan consent.ConsentNotify, 10)
	responsechan := make(chan consent.ConsentResponse, 10)
	stopchan := make(chan os.Signal, 1)
	var wg sync.WaitGroup
	var peermanager *store.Peermanager
	var peerconsent *consent.Consent
	var quiclisten *transfer.QListener
	var quicsend *transfer.QSender

	if peermanager = store.Getpeermanager(); peermanager == nil {
		fmt.Fprintln(os.Stdout, "Failed to initialize peer manager")
		os.Exit(0)
	}

	if peerconsent = consent.Getconsent(); peerconsent != nil {
		peerconsent.SetupNotify(notifychan, responsechan)
	} else {
		fmt.Fprintln(os.Stdout, "Failed to initialize consent handler")
		os.Exit(0)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		peerconsent.Receiveconsent()
	}()

	if quiclisten, quicsend = transfer.Getfileshare(); quiclisten == nil || quicsend == nil {
		fmt.Fprintln(os.Stdout, "Failed to initialise QUIC listener & sender")
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		quiclisten.QUICListener(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		CLI(notifychan, responsechan)
	}()

	<-stopchan
	fmt.Fprintln(os.Stdout, "Shutdown signal received, cleaning up...")
	cancel()
	close(notifychan)
	close(responsechan)
	wg.Wait()
	os.Exit(0)
}
