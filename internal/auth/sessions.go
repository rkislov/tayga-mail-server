package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/tayga/tms/internal/storage"
	"sync"
	"time"
)

type SessionInfo struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Protocol   string    `json:"protocol"`
	Address    string    `json:"address"`
	Agent      string    `json:"agent"`
	Started    time.Time `json:"started_at"`
	Seen       time.Time `json:"last_seen"`
	Connection bool      `json:"connection"`
}
type activeSession struct {
	SessionInfo
	close func() error
}
type SessionRegistry struct {
	mu    sync.Mutex
	items map[string]activeSession
}

func (a *Layer) TrackSession(user *storage.User, protocol, address, agent, key string, connection bool, close func() error) string {
	sum := sha256.Sum256([]byte(protocol + "\x00" + key))
	id := hex.EncodeToString(sum[:])
	a.sessions.mu.Lock()
	defer a.sessions.mu.Unlock()
	if a.sessions.items == nil {
		a.sessions.items = map[string]activeSession{}
	}
	now := time.Now().UTC()
	entry, exists := a.sessions.items[id]
	if !exists {
		entry.SessionInfo = SessionInfo{ID: id, UserID: user.ID, Email: user.Email, Protocol: protocol, Address: address, Agent: agent, Started: now, Connection: connection}
	}
	entry.Seen = now
	entry.close = close
	a.sessions.items[id] = entry
	return id
}
func (a *Layer) ForgetSession(id string) {
	a.sessions.mu.Lock()
	delete(a.sessions.items, id)
	a.sessions.mu.Unlock()
}
func (a *Layer) Sessions() []SessionInfo {
	a.sessions.mu.Lock()
	defer a.sessions.mu.Unlock()
	out := []SessionInfo{}
	for id, item := range a.sessions.items {
		if !item.Connection && time.Since(item.Seen) > 15*time.Minute {
			delete(a.sessions.items, id)
			continue
		}
		out = append(out, item.SessionInfo)
	}
	return out
}
func (a *Layer) SessionOwner(id string) string {
	a.sessions.mu.Lock()
	defer a.sessions.mu.Unlock()
	return a.sessions.items[id].UserID
}
func (a *Layer) CloseSession(id string) error {
	a.sessions.mu.Lock()
	entry, ok := a.sessions.items[id]
	a.sessions.mu.Unlock()
	if !ok {
		return storage.ErrNotFound
	}
	if entry.close == nil {
		return ErrUnsupportedSource
	}
	if err := entry.close(); err != nil {
		return err
	}
	a.ForgetSession(id)
	return nil
}
func (a *Layer) TrackToken(user *storage.User, token, address, agent string) {
	a.TrackSession(user, "Web", address, agent, token, false, func() error { return a.Tokens.RevokeAccess(context.Background(), token) })
}
