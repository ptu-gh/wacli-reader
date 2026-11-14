package jid

import "testing"

// The expected values match whatsmeow's types.ParseJID / JID.String, which this
// package replaces; see the package comment.
func TestParseJIDRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    JID
		nonAD   string
		wantErr bool
	}{
		{in: "15551234567@s.whatsapp.net", want: JID{User: "15551234567", Server: DefaultUserServer}, nonAD: "15551234567@s.whatsapp.net"},
		{in: "15551234567:12@s.whatsapp.net", want: JID{User: "15551234567", Device: 12, Server: DefaultUserServer}, nonAD: "15551234567@s.whatsapp.net"},
		{in: "15551234567.1:2@s.whatsapp.net", want: JID{User: "15551234567", RawAgent: 1, Device: 2, Server: DefaultUserServer}, nonAD: "15551234567@s.whatsapp.net"},
		{in: "123456789@lid", want: JID{User: "123456789", Server: HiddenUserServer}, nonAD: "123456789@lid"},
		{in: "120363000000000001@g.us", want: JID{User: "120363000000000001", Server: GroupServer}, nonAD: "120363000000000001@g.us"},
		{in: "status@broadcast", want: JID{User: "status", Server: BroadcastServer}, nonAD: "status@broadcast"},
		{in: "g.us", want: JID{Server: GroupServer}, nonAD: "g.us"},
		{in: "1.2.3@x", wantErr: true},
		{in: "1:2:3@x", wantErr: true},
		{in: "1:x@y", wantErr: true},
	} {
		got, err := ParseJID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseJID(%q) = %+v, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseJID(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
			continue
		}
		if s := got.String(); s != tc.in {
			t.Errorf("ParseJID(%q).String() = %q", tc.in, s)
		}
		if s := got.ToNonAD().String(); s != tc.nonAD {
			t.Errorf("ParseJID(%q).ToNonAD() = %q, want %q", tc.in, s, tc.nonAD)
		}
	}
}

func TestEmptyJID(t *testing.T) {
	var j JID
	if !j.IsEmpty() || j.String() != "" {
		t.Fatalf("zero JID: IsEmpty=%v String=%q", j.IsEmpty(), j.String())
	}
}

func TestParseUserOrJID(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		wantErr  bool
	}{
		{in: "+1 (555) 123-4567", want: "15551234567@s.whatsapp.net"},
		{in: "  15551234567  ", want: "15551234567@s.whatsapp.net"},
		{in: "123456789@lid", want: "123456789@lid"},
		{in: "", wantErr: true},
		{in: "12345", wantErr: true},
		{in: "1555+1234567", wantErr: true},
		{in: "Alice", wantErr: true},
	} {
		got, err := ParseUserOrJID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseUserOrJID(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got.String() != tc.want {
			t.Errorf("ParseUserOrJID(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}
