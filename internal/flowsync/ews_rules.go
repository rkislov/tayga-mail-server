// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/sieve"
	"github.com/tayga/tms/internal/storage"
)

type inboxRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
	XML      string `json:"xml"`
}

func (h *ewsHandler) inboxRulesEWS(ctx context.Context, u *storage.User, op, body string) (string, error) {
	fail := func(code, message string) (string, error) { return ewsDirectResponse(op, code, message, ""), nil }
	root, err := parseProtocolXML([]byte(body))
	if err != nil {
		return "", err
	}
	req := root.find(op)
	if req == nil {
		return fail("ErrorInvalidRequest", "Missing operation")
	}
	if mailbox := strings.TrimSpace(req.value("MailboxSmtpAddress")); mailbox != "" && !strings.EqualFold(mailbox, u.Email) {
		return fail("ErrorAccessDenied", "Only your own mailbox rules are available")
	}
	saved, err := h.store.GetInboxRules(ctx, u.ID)
	if err != nil {
		return "", err
	}
	var rules []inboxRule
	if err = json.Unmarshal([]byte(saved.RulesJSON), &rules); err != nil {
		return "", err
	}
	if op == "GetInboxRules" {
		var b strings.Builder
		for _, rule := range rules {
			b.WriteString(rule.XML)
		}
		return ewsDirectResponse(op, "NoError", "", "<m:OutlookRuleBlobExists>false</m:OutlookRuleBlobExists><m:InboxRules>"+b.String()+"</m:InboxRules>"), nil
	}
	operations := req.child("Operations")
	if operations == nil || len(operations.Children) == 0 || len(operations.Children) > 100 {
		return fail("ErrorInvalidRequest", "Provide 1–100 operations")
	}
	for _, operation := range operations.Children {
		node := operation.child("Rule")
		id := operation.value("RuleId")
		if node != nil {
			id = node.value("RuleId")
		}
		index := -1
		for i := range rules {
			if rules[i].ID == id {
				index = i
				break
			}
		}
		switch operation.Name.Local {
		case "DeleteRuleOperation":
			if index < 0 {
				return fail("ErrorInboxRulesValidationError", "Rule not found")
			}
			rules = append(rules[:index], rules[index+1:]...)
		case "CreateRuleOperation", "SetRuleOperation":
			if node == nil {
				return fail("ErrorInvalidRequest", "Rule is required")
			}
			if operation.Name.Local == "SetRuleOperation" && index < 0 {
				return fail("ErrorInboxRulesValidationError", "Rule not found")
			}
			if operation.Name.Local == "CreateRuleOperation" {
				if id != "" {
					return fail("ErrorInvalidRequest", "New rules must not provide RuleId")
				}
				id = storage.NewID()
			}
			rule, _, e := h.compileInboxRule(ctx, u, node, id)
			if e != nil {
				return fail("ErrorInboxRulesValidationError", e.Error())
			}
			if index < 0 {
				rules = append(rules, rule)
			} else {
				rules = append(rules[:index], rules[index+1:]...)
				rules = append(rules, rule)
			}
			// Insert into the requested priority, shifting other rules without losing order.
			last := len(rules) - 1
			pos := rule.Priority - 1
			if pos > last {
				pos = last
			}
			copy(rules[pos+1:], rules[pos:last])
			rules[pos] = rule
		default:
			return fail("ErrorInvalidRequest", "Unsupported rule operation")
		}
	}
	if len(rules) > 100 {
		return fail("ErrorInboxRulesValidationError", "Maximum 100 rules")
	}
	var script strings.Builder
	script.WriteString("# Managed by Outlook / EWS. Runs before the active personal Sieve script.\nrequire [\"fileinto\", \"copy\", \"imap4flags\", \"body\"];\n")
	for i := range rules {
		node, e := parseProtocolXML([]byte(rules[i].XML))
		if e != nil {
			return "", e
		}
		node = node.find("Rule")
		node.child("Priority").Text = strconv.Itoa(i + 1)
		rule, source, e := h.compileInboxRule(ctx, u, node, rules[i].ID)
		if e != nil {
			return fail("ErrorInboxRulesValidationError", e.Error())
		}
		rules[i] = rule
		if rule.Enabled {
			script.WriteString(source)
		}
	}
	if err = sieve.CheckScript(script.String()); err != nil {
		return fail("ErrorInboxRulesValidationError", "Rule cannot be represented by the server's Sieve engine")
	}
	encoded, err := json.Marshal(rules)
	if err != nil {
		return "", err
	}
	if err = h.store.SaveInboxRules(ctx, u.ID, saved.Revision, string(encoded), script.String()); errors.Is(err, storage.ErrRulesConflict) {
		return fail("ErrorIrresolvableConflict", err.Error())
	} else if err != nil {
		return "", err
	}
	return ewsDirectResponse(op, "NoError", "", ""), nil
}
func sieveString(v string) (string, error) {
	if len(v) > 4096 || strings.ContainsAny(v, "\x00\r\n") {
		return "", fmt.Errorf("Invalid rule text")
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`, nil
}
func ruleBool(v string) (bool, error) {
	switch v {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("Expected boolean")
	}
}
func (h *ewsHandler) compileInboxRule(ctx context.Context, u *storage.User, n *protocolNode, id string) (inboxRule, string, error) {
	fail := func(msg string) (inboxRule, string, error) { return inboxRule{}, "", errors.New(msg) }
	allowed := map[string]bool{"RuleId": true, "DisplayName": true, "Priority": true, "IsEnabled": true, "IsNotSupported": true, "IsInError": true, "Conditions": true, "Exceptions": true, "Actions": true}
	seen := map[string]bool{}
	for _, c := range n.Children {
		if !allowed[c.Name.Local] || seen[c.Name.Local] {
			return fail("Unsupported or repeated rule property: " + c.Name.Local)
		}
		seen[c.Name.Local] = true
	}
	name := strings.TrimSpace(n.value("DisplayName"))
	if name == "" || len([]rune(name)) > 256 {
		return fail("Rule name must contain 1–256 characters")
	}
	priority, err := strconv.Atoi(n.value("Priority"))
	if err != nil || priority < 1 {
		return fail("Priority must be positive")
	}
	enabled, err := ruleBool(n.value("IsEnabled"))
	if err != nil {
		return fail("IsEnabled is required")
	}
	for _, property := range []string{"IsNotSupported", "IsInError"} {
		if node := n.child(property); node != nil {
			value, e := ruleBool(node.Text)
			if e != nil || value {
				return fail("Unsupported rule")
			}
		}
	}
	cond, err := rulePredicates(n.child("Conditions"), u.Email, false)
	if err != nil {
		return fail(err.Error())
	}
	except, err := rulePredicates(n.child("Exceptions"), u.Email, true)
	if err != nil {
		return fail(err.Error())
	}
	if except != "false" {
		cond = "allof (" + cond + ", not " + except + ")"
	}
	actions := n.child("Actions")
	if actions == nil || len(actions.Children) == 0 {
		return fail("At least one action is required")
	}
	flags, filing, stop := "", "", ""
	seen = map[string]bool{}
	moves := 0
	for _, a := range actions.Children {
		if seen[a.Name.Local] {
			return fail("Repeated action")
		}
		seen[a.Name.Local] = true
		switch a.Name.Local {
		case "MoveToFolder", "CopyToFolder":
			if len(a.Children) != 1 || (a.Children[0].Name.Local != "FolderId" && a.Children[0].Name.Local != "DistinguishedFolderId") || strings.TrimSpace(nodeAttr(a.Children[0], "Id")) == "" {
				return fail("Exactly one destination folder is required")
			}
			folderID, e := h.resolveEWSFolder(ctx, u, a.render())
			if e != nil {
				return fail("Destination folder is unavailable")
			}
			folder, e := h.store.GetMailboxByID(ctx, folderID)
			if e != nil || folder.UserID != u.ID {
				return fail("Destination must be your own mail folder")
			}
			quoted, e := sieveString(folder.Name)
			if e != nil {
				return fail(e.Error())
			}
			copyFlag := ""
			if a.Name.Local == "CopyToFolder" {
				copyFlag = ":copy "
			} else {
				moves++
			}
			filing += " fileinto " + copyFlag + quoted + ";\n"
			// Persist stable folder IDs instead of locale-dependent distinguished names.
			a.Children = []*protocolNode{{Name: xml.Name{Local: "FolderId"}, Attr: []xml.Attr{{Name: xml.Name{Local: "Id"}, Value: folder.ID}}}}
		case "Delete", "PermanentDelete", "MarkAsRead", "StopProcessingRules":
			on, e := ruleBool(a.Text)
			if e != nil {
				return fail(e.Error())
			}
			if !on {
				continue
			}
			switch a.Name.Local {
			case "Delete":
				moves++
				filing += " fileinto \"Trash\";\n"
			case "PermanentDelete":
				moves++
				filing += " discard;\n"
			case "MarkAsRead":
				flags = " addflag \"\\\\Seen\";\n"
			case "StopProcessingRules":
				stop = " stop;\n"
			}
		default:
			return fail("Unsupported action: " + a.Name.Local)
		}
	}
	if moves > 1 {
		return fail("Use one move or delete action")
	}
	if flags+filing+stop == "" {
		return fail("At least one action must be enabled")
	}
	// Normalize metadata to the EWS types namespace for stored round trips.
	var normalize func(*protocolNode)
	normalize = func(node *protocolNode) {
		node.Name.Space = "http://schemas.microsoft.com/exchange/services/2006/types"
		for _, c := range node.Children {
			normalize(c)
		}
	}
	if n.child("RuleId") == nil {
		n.Children = append([]*protocolNode{{Name: xml.Name{Local: "RuleId"}, Text: id}}, n.Children...)
	} else {
		n.child("RuleId").Text = id
	}
	normalize(n)
	return inboxRule{ID: id, Name: name, Priority: priority, Enabled: enabled, XML: n.render()}, "if " + cond + " {\n" + flags + filing + stop + "}\n", nil
}
func rulePredicates(n *protocolNode, email string, exception bool) (string, error) {
	var tests []string
	if n != nil {
		seen := map[string]bool{}
		for _, c := range n.Children {
			if seen[c.Name.Local] {
				return "", fmt.Errorf("Repeated condition: %s", c.Name.Local)
			}
			seen[c.Name.Local] = true
			var test string
			switch c.Name.Local {
			case "ContainsSubjectStrings", "ContainsBodyStrings", "ContainsSubjectOrBodyStrings", "ContainsSenderStrings", "ContainsRecipientStrings", "FromAddresses", "SentToAddresses":
				var values []string
				for _, v := range c.Children {
					value := v.Text
					if c.Name.Local == "FromAddresses" || c.Name.Local == "SentToAddresses" {
						if v.Name.Local != "Address" {
							return "", errors.New("Invalid address condition")
						}
						value = v.value("EmailAddress")
						routing := v.value("RoutingType")
						if routing != "" && routing != "SMTP" {
							return "", errors.New("Only SMTP address conditions are supported")
						}
					} else if v.Name.Local != "String" {
						return "", errors.New("Invalid text condition")
					}
					if strings.TrimSpace(value) == "" {
						return "", errors.New("Empty condition value")
					}
					quoted, e := sieveString(value)
					if e != nil {
						return "", e
					}
					values = append(values, quoted)
				}
				if len(values) == 0 || len(values) > 50 {
					return "", errors.New("Provide 1–50 condition values")
				}
				list := "[" + strings.Join(values, ",") + "]"
				switch c.Name.Local {
				case "ContainsSubjectStrings":
					test = `header :contains :comparator "i;unicode-casemap" "Subject" ` + list
				case "ContainsBodyStrings":
					test = `body :text :contains :comparator "i;unicode-casemap" ` + list
				case "ContainsSubjectOrBodyStrings":
					test = `anyof (header :contains :comparator "i;unicode-casemap" "Subject" ` + list + `, body :text :contains :comparator "i;unicode-casemap" ` + list + `)`
				case "ContainsSenderStrings":
					test = `address :contains "From" ` + list
				case "ContainsRecipientStrings":
					test = `address :contains ["To","Cc"] ` + list
				case "FromAddresses":
					test = `address :is "From" ` + list
				case "SentToAddresses":
					test = `address :is ["To","Cc"] ` + list
				}
			case "SentToMe", "SentCcMe", "NotSentToMe":
				value, e := ruleBool(c.Text)
				if e != nil {
					return "", e
				}
				field := "To"
				if c.Name.Local == "SentCcMe" {
					field = "Cc"
				}
				quoted, e := sieveString(email)
				if e != nil {
					return "", e
				}
				test = `address :is "` + field + `" ` + quoted
				if c.Name.Local == "NotSentToMe" {
					test = "not " + test
				}
				if !value {
					test = "not " + test
				}
			default:
				return "", fmt.Errorf("Unsupported condition: %s", c.Name.Local)
			}
			tests = append(tests, test)
		}
	}
	if len(tests) == 0 {
		if exception {
			return "false", nil
		}
		return "true", nil
	}
	if len(tests) == 1 {
		return tests[0], nil
	}
	join := "allof"
	if exception {
		join = "anyof"
	}
	return join + " (" + strings.Join(tests, ", ") + ")", nil
}
