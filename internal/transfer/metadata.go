package transfer

import (
	"context"
	"net"
)

type FileInfo struct {
	Filename string // filename with extension
	Size     int64  // bytes totatl
}

type TransferStatus int

const (
	START TransferStatus = iota
	RESUME
	CANCEL
	PAUSED
)

type FileProgress struct {
	File             FileInfo
	BytesTransferred int64
	Percentage       float64
	Status           TransferStatus
}

type AppRole int

const (
	SENDER AppRole = iota
	RECEIVER
)

type FileTransferSession struct {
	File     FileInfo
	Progress FileProgress
	Conn     net.Conn
	Role     AppRole
	Ctx      context.Context
	Cancel   context.CancelFunc
}
