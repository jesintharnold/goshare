package transfer

import (
	"context"
	"fmt"
	"goshare/internal/discovery"
	"log"
	"net"
)

type TransferService struct {
	Peerinfo discovery.PeerInfo
	Session  []*FileTransferSession
}

func (ts *TransferService) ConnectToPeer() {
	peeraddress:=fmt.Sprintf("%s:%d",ts.Peerinfo.IPAddress,8080)
	context,cancel:=context.WithCancel(context.Background())
	conn,err:=net.Dial("tcp",peeraddress)
	if err!=nil{
		log.Printf("Failed to connect to peer : %v \n,error - %v",peeraddress,err)
	}
	log.Printf("Successfully connected to the peer : %v",peeraddress)
	fileCon:= &FileTransferSession{
		Conn: conn,
		Ctx: context,
		Cancel: cancel,
		Role: SENDER,
	}

	ts.Session = append(ts.Session, fileCon)
	fmt.Printf("File Transfer struct %v",ts.Session)
}