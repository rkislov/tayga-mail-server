// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"fmt"
	"github.com/tayga/tms/internal/storage"
	"strings"
)

func settingsResponse(u *storage.User, body []byte) (string, error) {
	doc, err := parseProtocolXML(body)
	if err != nil {
		return "", err
	}
	root := doc.find("Settings")
	if root == nil {
		return `<Settings xmlns="Settings:"><Status>2</Status></Settings>`, nil
	}
	var out strings.Builder
	out.WriteString(`<Settings xmlns="Settings:"><Status>1</Status>`)
	for _, request := range root.Children {
		switch request.Name.Local {
		case "DeviceInformation":
			status := 1
			if request.child("Set") == nil {
				status = 2
			}
			fmt.Fprintf(&out, `<DeviceInformation><Status>%d</Status></DeviceInformation>`, status)
		case "UserInformation":
			if request.child("Get") == nil {
				out.WriteString(`<UserInformation><Status>2</Status></UserInformation>`)
			} else {
				fmt.Fprintf(&out, `<UserInformation><Status>1</Status><Get><EmailAddresses><SMTPAddress>%s</SMTPAddress></EmailAddresses></Get></UserInformation>`, xmlEscape(u.Email))
			}
		default:
			return `<Settings xmlns="Settings:"><Status>2</Status></Settings>`, nil
		}
	}
	out.WriteString(`</Settings>`)
	return out.String(), nil
}
