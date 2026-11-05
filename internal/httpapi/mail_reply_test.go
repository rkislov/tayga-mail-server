package httpapi

import (
	"reflect"
	"testing"
)

func TestReplyAllRecipients(t *testing.T) {
	m := parsedMsg{From: "Sender <sender@ex.com>", ReplyTo: "Support <support@ex.com>", To: "Me <me@ex.com>, Other <other@ex.com>", Cc: "OTHER@ex.com, copy@ex.com, me@ex.com"}
	to, cc := replyRecipients(m, "ME@ex.com")
	if !reflect.DeepEqual(to, []string{"support@ex.com", "other@ex.com"}) || !reflect.DeepEqual(cc, []string{"copy@ex.com"}) {
		t.Fatalf("wrong recipients: %v %v", to, cc)
	}
}

func TestReplyAllExcludesOwnAliases(t *testing.T) {
	m := parsedMsg{From: "sender@outside.example", To: "support@alias.example, other@outside.example", Cc: "owner@alias.example, owner@example.com"}
	to, cc := replyRecipients(m, "owner@example.com", "support@alias.example", "owner@alias.example")
	if !reflect.DeepEqual(to, []string{"sender@outside.example", "other@outside.example"}) || len(cc) != 0 {
		t.Fatal(to, cc)
	}
}
