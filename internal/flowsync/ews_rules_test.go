// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
	"strings"
	"testing"
)

func TestInboxRulesRoundTripExecutionAndAtomicity(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	run := func(op, body string) string {
		t.Helper()
		out, e := h.inboxRulesEWS(ctx, u, op, "<"+op+">"+body+"</"+op+">")
		if e != nil {
			t.Fatal(e)
		}
		requireXML(t, out)
		return out
	}
	rule := `<Rule><DisplayName>Работа</DisplayName><Priority>1</Priority><IsEnabled>true</IsEnabled><Conditions><ContainsSubjectStrings><String>ВСТРЕЧА</String></ContainsSubjectStrings></Conditions><Exceptions><FromAddresses><Address><EmailAddress>skip@example.com</EmailAddress></Address></FromAddresses></Exceptions><Actions><MoveToFolder><DistinguishedFolderId Id="junkemail"/></MoveToFolder><MarkAsRead>true</MarkAsRead><StopProcessingRules>true</StopProcessingRules></Actions></Rule>`
	out := run("UpdateInboxRules", "<Operations><CreateRuleOperation>"+rule+"</CreateRuleOperation></Operations>")
	if !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
	listed := run("GetInboxRules", "")
	id := extractTag(listed, "RuleId")
	if id == "" || !strings.Contains(listed, "Работа") {
		t.Fatal(listed)
	}
	saved, err := st.GetInboxRules(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	custom := `require "fileinto"; fileinto "Archive";`
	for _, test := range []struct{ from, subject, folder string }{{"a@example.com", "встреча завтра", "Junk"}, {"skip@example.com", "встреча завтра", "Archive"}, {"a@example.com", "hello", "Archive"}} {
		raw := []byte("From: " + test.from + "\r\nTo: " + u.Email + "\r\nSubject: " + test.subject + "\r\n\r\nbody\r\n")
		result, e := sieve.ExecuteScripts([]string{saved.Script, custom}, test.from, u.Email, u.Email, raw)
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Deliveries) != 1 || result.Deliveries[0].Mailbox != test.folder {
			t.Fatalf("%+v %s", result, saved.Script)
		}
		if test.folder == "Junk" && !storage.HasFlag(strings.Join(result.Deliveries[0].Flags, " "), `\Seen`) {
			t.Fatal(result)
		}
	}
	bad := strings.Replace(rule, "<StopProcessingRules>true</StopProcessingRules>", "<ForwardToRecipients><Address><EmailAddress>x@outside.example</EmailAddress></Address></ForwardToRecipients>", 1)
	out = run("UpdateInboxRules", "<Operations><DeleteRuleOperation><RuleId>"+id+"</RuleId></DeleteRuleOperation><CreateRuleOperation>"+bad+"</CreateRuleOperation></Operations>")
	if !strings.Contains(out, "ErrorInboxRulesValidationError") {
		t.Fatal(out)
	}
	if after, _ := st.GetInboxRules(ctx, u.ID); after != saved {
		t.Fatal("partial update persisted")
	}
	if e := st.SaveInboxRules(ctx, u.ID, saved.Revision-1, "[]", ""); e != storage.ErrRulesConflict {
		t.Fatal("stale write accepted", e)
	}
	if out = run("GetInboxRules", "<MailboxSmtpAddress>other@example.com</MailboxSmtpAddress>"); !strings.Contains(out, "ErrorAccessDenied") {
		t.Fatal(out)
	}
	changed := strings.Replace(rule, "<DisplayName>", "<RuleId>"+id+"</RuleId><DisplayName>", 1)
	changed = strings.Replace(changed, "<IsEnabled>true", "<IsEnabled>false", 1)
	if out = run("UpdateInboxRules", "<Operations><SetRuleOperation>"+changed+"</SetRuleOperation></Operations>"); !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
	if out = run("UpdateInboxRules", "<Operations><DeleteRuleOperation><RuleId>"+id+"</RuleId></DeleteRuleOperation></Operations>"); !strings.Contains(out, "NoError") {
		t.Fatal(out)
	}
	if out = run("GetInboxRules", ""); strings.Contains(out, id) {
		t.Fatal(out)
	}
}
