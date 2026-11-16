package site

import "testing"

func TestASignInCreatesAccountsUntilTurnedOff(t *testing.T) {
	var records [][]byte
	s, err := New(nil, func(r []byte) error { records = append(records, r); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if s.SignIn().NoNewAccounts {
		t.Fatal("new accounts at a first sign-in are off by default; ADR-0030 keeps them on")
	}
	if err := s.SetSignIn("admin", SignIn{NoNewAccounts: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSignIn("admin", SignIn{NoNewAccounts: true}); err != nil || len(records) != 1 {
		t.Fatalf("an unchanged setting is journalled: %d records, %v", len(records), err)
	}
	again, err := New(records, func([]byte) error { return nil })
	if err != nil || !again.SignIn().NoNewAccounts {
		t.Fatalf("read again: %v", err)
	}
	if _, err := New([][]byte{[]byte(`{"v":1,"kind":"SignInSet"}`)}, nil); err == nil {
		t.Fatal("a sign-in event without its setting")
	}
}
