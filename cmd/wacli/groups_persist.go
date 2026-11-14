package main

import (
	types "github.com/openclaw/wacli/internal/jid"
)

func canonicalCLIJID(jid types.JID) types.JID {
	if jid.Server == types.DefaultUserServer {
		return jid.ToNonAD()
	}
	return jid
}

func groupKindLabel(isParent bool, linkedParentJID string) string {
	if isParent {
		return "community"
	}
	if linkedParentJID != "" {
		return "subgroup"
	}
	return "group"
}
