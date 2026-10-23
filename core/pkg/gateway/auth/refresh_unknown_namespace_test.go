package auth

import (
	"context"
	"errors"
	"testing"
)

// A refresh naming a namespace the registry does not hold (deleted, or never
// created) was reported as ErrRefreshTransient, so the handler answered 503
// "retry" and a client retried a session that can never be renewed. It is the
// refusal now, which the handler answers 401.
func TestRefreshToken_anUnknownNamespaceIsRefusedNotTransient(t *testing.T) {
	s, _, _ := realRegistry(t)
	grantEndUser(t, s)
	ctx := context.Background()
	_, refresh, _, err := s.IssueTokens(ctx, deviceOwner, "anchat")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	_, _, _, _, err = s.RefreshToken(ctx, refresh, "no-such-namespace", nil)
	if errors.Is(err, ErrRefreshTransient) {
		t.Fatalf("an unknown namespace was reported transient (503 retry): %v", err)
	}
	var unknown *ErrNoSuchNamespace
	if !errors.As(err, &unknown) || unknown.Namespace != "no-such-namespace" {
		t.Fatalf("refresh into an unknown namespace = %v, want ErrNoSuchNamespace", err)
	}
}
