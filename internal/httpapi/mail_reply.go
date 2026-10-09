package httpapi

import (
	"net/mail"
	"strings"
)

func replyRecipients(message parsedMsg, self string) (to, cc []string) {
	seen := map[string]bool{strings.ToLower(self): true}
	collect := func(value string, dst *[]string) {
		addresses, err := mail.ParseAddressList(value)
		if err != nil {
			return
		}
		for _, address := range addresses {
			email := strings.ToLower(address.Address)
			if !seen[email] {
				seen[email] = true
				*dst = append(*dst, address.Address)
			}
		}
	}
	reply := message.ReplyTo
	if reply == "" {
		reply = message.From
	}
	collect(reply, &to)
	collect(message.To, &to)
	collect(message.Cc, &cc)
	return
}
