package fileshare

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type TransferStatus int

const (
	PENDING TransferStatus = iota
	TRANSFERRING
	PAUSED
	COMPLETED
	FAILED
	CANCELLED
)

type TransferDirection int

const (
	SENDING TransferDirection = iota
	RECEIVING
)

type FileInfo struct {
	Filename string `json:"filename"` //
	Size     int64  `json:"size"`     // In bytes
}

type FileTransfer struct {
	FileInfo         FileInfo
	Direction        TransferDirection
	Status           TransferStatus
	BytesTransferred int64
	StartTime        time.Time
	LastUpdateTime   time.Time
	Error            error
	speed            float64
	Progress         float64
}

//each peer has a session manager for each connection with other peer
//currently we are focusing on 1-1 Peer alone
//Later we will focus on Multi peer file share

type SessionManager struct {
	mu              sync.Mutex
	activeTransfers map[string]*FileTransfer //we are mapping key as filename and Filemetadata/progress as values
	ctx             context.Context
	cancel          context.CancelFunc
}

func (sm *SessionManager) CreateTransfer(info FileInfo, direction TransferDirection) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	transferKey := info.Filename
	transfer := &FileTransfer{
		FileInfo:       info,
		Direction:      direction,
		Status:         PENDING,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
		Progress:       0,
	}
	sm.activeTransfers[string(transferKey)] = transfer
	return transferKey
}
func (sm *SessionManager) UpdateTransferProgress(transferID string, bytestransferred int64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	transfer, exists := sm.activeTransfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found %s", transferID)
	}

	now := time.Now()
	timediff := now.Sub(transfer.LastUpdateTime).Seconds()
	if timediff > 0 {
		filesent := bytestransferred - transfer.BytesTransferred
		transfer.speed = float64(filesent) / timediff
	}
	transfer.LastUpdateTime = now
	transfer.BytesTransferred = bytestransferred
	transfer.Progress = float64(bytestransferred) / float64(transfer.FileInfo.Size) * 100
	transfer.Status = TRANSFERRING
	log.Printf("%s, progress - %.2f%%", transferID, transfer.Progress)
	return nil
}
func (sm *SessionManager) CompleteTransfer(transferID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	transfer, exists := sm.activeTransfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found %s", transferID)
	}
	transfer.LastUpdateTime = time.Now()
	transfer.Progress = 100
	transfer.Status = COMPLETED
	log.Printf("%s - sent successfully", transferID)
	return nil
}
func (sm *SessionManager) FailTransfer(transferID string, err error) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	transfer, exists := sm.activeTransfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found %s", transferID)
	}
	transfer.Status = FAILED
	transfer.Error = err
	transfer.LastUpdateTime = time.Now()
	return nil
}

// Initate new session for creating Invidual peer to peer file transfer
func NewSession(ctx context.Context) *SessionManager {
	sctx, cancel := context.WithCancel(ctx)
	return &SessionManager{
		activeTransfers: make(map[string]*FileTransfer),
		ctx:             sctx,
		cancel:          cancel,
	}
}
