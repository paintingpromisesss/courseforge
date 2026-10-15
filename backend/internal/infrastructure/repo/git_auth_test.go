package repo

import (
	"context"
	"testing"
)

func TestGitAuthRoundTrip(t *testing.T) {
	r := NewGitAuthRepository(t.TempDir())
	a, err := r.Load(context.Background())
	if err != nil || a != nil {
		t.Fatalf("empty load = %v, %v", a, err)
	}
	if err := r.Save(context.Background(), &GitAuth{Token: "ghp_abc123def456", Username: "me"}); err != nil {
		t.Fatal(err)
	}
	a, err = r.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if a.Token != "ghp_abc123def456" || a.Username != "me" {
		t.Fatalf("round trip: %+v", a)
	}
	if err := r.Delete(); err != nil {
		t.Fatal(err)
	}
	a, err = r.Load(context.Background())
	if err != nil || a != nil {
		t.Fatalf("after delete: %v, %v", a, err)
	}
	// Delete on absent file is not an error
	if err := r.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestMaskToken(t *testing.T) {
	if m := MaskToken("ghp_abcdefghijklmnop"); m != "ghp_…mnop" {
		t.Fatalf("mask = %s", m)
	}
	if MaskToken("short") != "****" {
		t.Fatal("short mask wrong")
	}
	if MaskToken("") != "" {
		t.Fatal("empty mask should be empty")
	}
}
