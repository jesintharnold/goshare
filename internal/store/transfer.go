package store

import (
	"fmt"
	"net"
	"sync"

	"golang.org/x/net/quic"
)

type Peer struct {
	ID          string
	IP          string
	QuicConn    quic.Conn
	FileSession *SessionManager
	TCPConn     net.Conn
	consent     bool
}

type Peermanager struct {
	peers map[string]*Peer
	mu    sync.Mutex
}

// SINGLETON Implementation in go
var (
	manager   *Peermanager
	singleton sync.Once
)

func Getpeermanager() *Peermanager {
	singleton.Do(func() {
		manager = &Peermanager{
			peers: make(map[string]*Peer),
		}
	})
	return manager
}

func (pm *Peermanager) Addpeer(peer *Peer) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, exists := pm.peers[peer.IP]
	if exists {
		return fmt.Errorf("peer with IP %s already exists", peer.IP)
	}
	pm.peers[peer.IP] = peer
	return nil
}
func (pm *Peermanager) Deletepeer(ip string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, exists := pm.peers[ip]
	if !exists {
		return fmt.Errorf("peer with IP %s does not found", ip)
	}
	delete(pm.peers, ip)
	return nil
}
func (pm *Peermanager) Getpeer(ip string) (*Peer, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	peer, exists := pm.peers[ip]
	if !exists {
		return nil, fmt.Errorf("peer with IP %s does not found", ip)
	}
	return peer, nil
}

func (pm *Peermanager) Changeconsent(ip string, val bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	peer, exists := pm.peers[ip]
	if !exists {
		return fmt.Errorf("peer with IP %s does not found", ip)
	}
	peer.consent = val
	return nil
}

// FOR A SINGLE PEER , WE NEED TO INITATE THESE
//
// ROLE : RECIVER
// - CREATE A NEW LISTENING CONSENT  SERVICE
//    - THIS WILL MAP IF THE USER IS CONSENTED OR NOT
//    - WE NEED TO VERIFY USING CHECKUSER
// - CREATE A QUIC LISTENER FOR LISTENING THE SERVICES
//    - IF ANY REQUEST IS COMING, WE NEED TO CHECK WITH CONSENT MAP
// 	  - YES ---> ACCEPT THE CONNECTION , CREATE A NEW PEER STATUS AND ALL
//    - NO  ---> SEND THE CONSENT RESPONSE (TO OTHER USER THAT CONSENT IS NOT GIVEN)
//
