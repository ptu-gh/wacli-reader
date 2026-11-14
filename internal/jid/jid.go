// Package jid is wacli-reader's whatsmeow-free stand-in for the two
// identifier APIs upstream's read commands use: go.mau.fi/whatsmeow/types
// (the JID struct, its parser and the server constants) and
// internal/wa.ParseUserOrJID.
//
// The fork links no WhatsApp client, so it cannot import either package.
// Files taken from upstream import this package under both names:
//
//	types "github.com/openclaw/wacli/internal/jid"
//	wa "github.com/openclaw/wacli/internal/jid"
//
// which keeps their bodies identical to upstream's. Only the subset those
// files call is provided; anything else upstream starts using fails to compile,
// which is the signal to extend this package rather than to import whatsmeow.
//
// Parsing and formatting follow whatsmeow's types.ParseJID and JID.String
// exactly, so a JID read from wacli.db, round-tripped through this package,
// compares equal to the string upstream's writer stored.
package jid

import (
	"fmt"
	"strconv"
	"strings"
)

// Known JID servers.
const (
	DefaultUserServer = "s.whatsapp.net"
	GroupServer       = "g.us"
	LegacyUserServer  = "c.us"
	BroadcastServer   = "broadcast"
	HiddenUserServer  = "lid"
	MessengerServer   = "msgr"
	InteropServer     = "interop"
	NewsletterServer  = "newsletter"
	HostedServer      = "hosted"
	HostedLIDServer   = "hosted.lid"
	BotServer         = "bot"
)

// JID is a WhatsApp identifier: a user on a server, optionally narrowed to one
// of the user's devices (an "AD" JID: user, agent and device).
type JID struct {
	User       string
	RawAgent   uint8
	Device     uint16
	Integrator uint16
	Server     string
}

// NewJID returns a regular (non-device) JID.
func NewJID(user, server string) JID {
	return JID{User: user, Server: server}
}

// ParseJID parses "server", "user@server", "user:device@server" or
// "user.agent:device@server".
func ParseJID(s string) (JID, error) {
	user, server, found := strings.Cut(s, "@")
	if !found {
		return NewJID("", s), nil
	}
	if strings.Contains(server, "@") {
		// whatsmeow ignores everything after a second '@'.
		server, _, _ = strings.Cut(server, "@")
	}
	j := JID{User: user, Server: server}
	switch {
	case strings.Contains(user, "."):
		name, ad, _ := strings.Cut(user, ".")
		if strings.Contains(ad, ".") {
			return j, fmt.Errorf("unexpected number of dots in JID")
		}
		j.User = name
		agentPart, devicePart, hasDevice := strings.Cut(ad, ":")
		if strings.Contains(devicePart, ":") {
			return j, fmt.Errorf("unexpected number of colons in JID")
		}
		agent, err := strconv.Atoi(agentPart)
		if err != nil {
			return j, fmt.Errorf("failed to parse device from JID: %w", err)
		}
		j.RawAgent = uint8(agent)
		if hasDevice {
			device, err := strconv.Atoi(devicePart)
			if err != nil {
				return j, fmt.Errorf("failed to parse device from JID: %w", err)
			}
			j.Device = uint16(device)
		}
	case strings.Contains(user, ":"):
		name, devicePart, _ := strings.Cut(user, ":")
		if strings.Contains(devicePart, ":") {
			return j, fmt.Errorf("unexpected number of colons in JID")
		}
		j.User = name
		device, err := strconv.Atoi(devicePart)
		if err != nil {
			return j, fmt.Errorf("failed to parse device from JID: %w", err)
		}
		j.Device = uint16(device)
	}
	return j, nil
}

// String formats the JID so that ParseJID reads it back unchanged.
func (j JID) String() string {
	switch {
	case j.Server == "":
		return ""
	case j.RawAgent > 0:
		return fmt.Sprintf("%s.%d:%d@%s", j.User, j.RawAgent, j.Device, j.Server)
	case j.Device > 0:
		return fmt.Sprintf("%s:%d@%s", j.User, j.Device, j.Server)
	case j.User != "":
		return j.User + "@" + j.Server
	default:
		return j.Server
	}
}

// ToNonAD drops the agent and device, leaving the user-level JID.
func (j JID) ToNonAD() JID {
	return JID{User: j.User, Server: j.Server, Integrator: j.Integrator}
}

// IsEmpty reports whether the JID has no server, which every valid JID has.
func (j JID) IsEmpty() bool {
	return j.Server == ""
}

// ParseUserOrJID is upstream's internal/wa.ParseUserOrJID: a string containing
// '@' is parsed as a JID, anything else as a phone number on the default user
// server.
func ParseUserOrJID(s string) (JID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return JID{}, fmt.Errorf("recipient is required")
	}
	if strings.Contains(s, "@") {
		return ParseJID(s)
	}
	phone, err := normalizePhoneRecipient(s)
	if err != nil {
		return JID{}, err
	}
	return JID{User: phone, Server: DefaultUserServer}, nil
}

func normalizePhoneRecipient(s string) (string, error) {
	var phone strings.Builder
	for i, ch := range s {
		switch {
		case ch == '+' && i == 0:
			continue
		case ch >= '0' && ch <= '9':
			phone.WriteRune(ch)
		case ch == ' ' || ch == '-' || ch == '(' || ch == ')' || ch == '.':
			continue
		default:
			return "", fmt.Errorf("invalid phone number %q: must contain digits, common formatting, or one leading +", s)
		}
	}
	normalized := phone.String()
	if normalized == "" {
		return "", fmt.Errorf("recipient is required")
	}
	if len(normalized) < 7 || len(normalized) > 15 {
		return "", fmt.Errorf("invalid phone number %q: must be 7-15 digits", s)
	}
	return normalized, nil
}
