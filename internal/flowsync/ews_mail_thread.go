// SPDX-License-Identifier: Apache-2.0
package flowsync

import (
	"fmt"
	"strings"
)

// Native Thunderbird builds its local threading headers from these EWS fields,
// before the MIME body is downloaded. Never substitute the storage UUID for the
// RFC Message-ID: replies refer to the identifier supplied by the sender.
func ewsMailThreadFields(messageID, references, inReplyTo string) string {
	var out strings.Builder
	if messageID != "" {
		fmt.Fprintf(&out, "<t:InternetMessageId>%s</t:InternetMessageId>", xmlEscape(messageID))
	}
	if references != "" {
		fmt.Fprintf(&out, "<t:References>%s</t:References>", xmlEscape(references))
	}
	if inReplyTo != "" {
		fmt.Fprintf(&out, `<t:InternetMessageHeaders><t:InternetMessageHeader HeaderName="In-Reply-To">%s</t:InternetMessageHeader></t:InternetMessageHeaders>`, xmlEscape(inReplyTo))
	}
	return out.String()
}
