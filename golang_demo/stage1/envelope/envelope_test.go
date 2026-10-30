package envelope

import (
	"errors"
	"testing"
)

// Compile-time check that AllowList is a Checker.
var _ Checker = AllowList{}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		wantErr error
	}{
		{name: "ok", msg: Message{From: "alice", Body: "ping"}},
		{name: "empty from", msg: Message{Body: "ping"}, wantErr: ErrEmptyFrom},
		{name: "blank from", msg: Message{From: "  ", Body: "ping"}, wantErr: ErrEmptyFrom},
		{name: "empty body", msg: Message{From: "alice"}, wantErr: ErrEmptyBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllowListCheck(t *testing.T) {
	check := NewAllowList("alice", "bob")
	if err := check.Check(Message{From: "alice", Body: "ping"}); err != nil {
		t.Fatalf("allowed peer: %v", err)
	}
	err := check.Check(Message{From: "eve", Body: "ping"})
	if !errors.Is(err, ErrBadPeer) {
		t.Fatalf("Check() error = %v, want %v", err, ErrBadPeer)
	}
	err = check.Check(Message{From: "alice"})
	if !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("Check() error = %v, want %v", err, ErrEmptyBody)
	}
}
