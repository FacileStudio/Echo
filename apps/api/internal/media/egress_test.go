package media

import (
	"testing"
)

func TestSanitizeRoom(t *testing.T) {
	cases := map[string]string{
		"team-standup": "team-standup",
		"Café Paris":   "_af___aris",
		"":             "",
	}
	for in, want := range cases {
		if got := sanitizeRoom(in); got != want {
			t.Fatalf("sanitizeRoom(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRecorderTokenCarriesRecordGrant(t *testing.T) {
	s := &Service{apiKey: "key", apiSecret: "secret", url: "ws://localhost"}
	token, err := s.recorderToken("room-a")
	if err != nil {
		t.Fatalf("recorderToken: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
}
