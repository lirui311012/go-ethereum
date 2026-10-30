// Package envelope is the message exchanged by the two peer processes.
package envelope

import (
	"errors"
	"fmt"
	"strings"
)

// Message is one JSON object on the wire.
type Message struct {
	From string `json:"from"`
	Body string `json:"body"`
	// Sum 由服务端在核对请求之后填上。
	// 客户端再算一遍，对不上就拒绝。
	Sum string `json:"sum,omitempty"`
}

// Checker accepts or rejects a message before a reply is sent.
type Checker interface {
	Check(Message) error
}

var (
	ErrEmptyFrom = errors.New("empty from")
	ErrEmptyBody = errors.New("empty body")
	ErrBadPeer   = errors.New("unexpected peer")
)

// Validate checks fields that every message must have.
func (m Message) Validate() error {
	if strings.TrimSpace(m.From) == "" {
		return fmt.Errorf("validate message: %w", ErrEmptyFrom)
	}
	if strings.TrimSpace(m.Body) == "" {
		return fmt.Errorf("validate message: %w", ErrEmptyBody)
	}
	return nil
}

// AllowList accepts messages whose From was passed to NewAllowList.
type AllowList struct {
	peers map[string]struct{}
}

// NewAllowList builds a Checker that allows only the named peers.
func NewAllowList(peers ...string) AllowList {
	set := make(map[string]struct{}, len(peers))
	for _, peer := range peers {
		set[peer] = struct{}{}
	}
	return AllowList{peers: set}
}

// Check reports an error when the message is incomplete or From is not allowed.
func (a AllowList) Check(m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if _, ok := a.peers[m.From]; !ok {
		return fmt.Errorf("check message from %q: %w", m.From, ErrBadPeer)
	}
	return nil
}
