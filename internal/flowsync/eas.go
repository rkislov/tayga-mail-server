package flowsync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// ActiveSync-compatible endpoint handled by proprietary FlowSync engine.
type easHandler struct {
	store storage.Driver
	ms    *mailstore.Store
}

func (h *easHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("MS-Server-ActiveSync", "18.0")
	w.Header().Set("MS-ASProtocolVersions", "14.0,14.1,16.0,16.1")
	w.Header().Set("MS-ASProtocolCommands", "FolderSync,Sync,Ping,Provision,GetItemEstimate,Options")
	w.Header().Set("X-FlowSync", "Tayga-Proprietary")
	w.Header().Set("X-FlowSync-Engine", "FlowSync/1.0")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u, ok := userFrom(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	cmd := r.URL.Query().Get("Cmd")
	deviceID := r.URL.Query().Get("DeviceId")
	if deviceID == "" {
		deviceID = "unknown"
	}
	deviceType := r.URL.Query().Get("DeviceType")
	dev, err := h.store.EnsureFlowSyncDevice(r.Context(), u.ID, deviceID, deviceType)
	if err != nil {
		http.Error(w, "device error", http.StatusInternalServerError)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))

	var out string
	switch strings.ToLower(cmd) {
	case "foldersync":
		out, err = h.folderSync(r.Context(), u, dev)
	case "sync":
		out, err = h.syncMail(r.Context(), u, dev, string(body))
	case "provision":
		out, err = h.provision(r.Context(), u, dev)
	case "ping":
		out = `<?xml version="1.0" encoding="utf-8"?><Ping xmlns="Ping:"><Status>1</Status></Ping>`
	case "getitemestimate":
		out, err = h.itemEstimate(r.Context(), u)
	default:
		http.Error(w, "unsupported command", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(out))
}

func (h *easHandler) folderSync(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice) (string, error) {
	mbs, err := h.store.ListMailboxes(ctx, u.ID)
	if err != nil {
		return "", err
	}
	key, err := h.store.GetFlowSyncSyncKey(ctx, dev.ID, "hierarchy")
	if err != nil {
		return "", err
	}
	next := nextSyncKey(key)
	if err := h.store.SetFlowSyncSyncKey(ctx, dev.ID, "hierarchy", next); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<FolderSync xmlns="FolderHierarchy:">`)
	b.WriteString(`<Status>1</Status>`)
	fmt.Fprintf(&b, `<SyncKey>%s</SyncKey>`, xmlEscape(next))
	b.WriteString(`<Changes>`)
	fmt.Fprintf(&b, `<Count>%d</Count>`, len(mbs))
	for _, mb := range mbs {
		typ := folderType(mb.Name)
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(mb.ID)) // mailbox UUID
		b.WriteString(`<ParentId>0</ParentId>`)
		fmt.Fprintf(&b, `<DisplayName>%s</DisplayName>`, xmlEscape(mb.Name))
		fmt.Fprintf(&b, `<Type>%d</Type>`, typ)
		b.WriteString(`</Add>`)
	}
	b.WriteString(`</Changes></FolderSync>`)
	return b.String(), nil
}

func (h *easHandler) syncMail(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, reqBody string) (string, error) {
	collectionID := extractTag(reqBody, "CollectionId")
	if collectionID == "" {
		mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
		if err != nil {
			return "", err
		}
		collectionID = mb.ID
	}
	mb, err := h.mailboxByID(ctx, u.ID, collectionID)
	if err != nil {
		return "", err
	}
	key, err := h.store.GetFlowSyncSyncKey(ctx, dev.ID, collectionID)
	if err != nil {
		return "", err
	}
	next := nextSyncKey(key)
	if err := h.store.SetFlowSyncSyncKey(ctx, dev.ID, collectionID, next); err != nil {
		return "", err
	}

	msgs, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return "", err
	}
	window := 25
	if len(msgs) > window {
		msgs = msgs[len(msgs)-window:]
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Sync xmlns="AirSync:"><Collections><Collection>`)
	fmt.Fprintf(&b, `<SyncKey>%s</SyncKey>`, xmlEscape(next))
	fmt.Fprintf(&b, `<CollectionId>%s</CollectionId>`, xmlEscape(collectionID))
	b.WriteString(`<Status>1</Status><Commands>`)
	for _, m := range msgs {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(m.ID)) // message UUID
		b.WriteString(`<ApplicationData>`)
		fmt.Fprintf(&b, `<Email:Subject xmlns:Email="Email:">%s</Email:Subject>`, xmlEscape(fmt.Sprintf("Message UID %d", m.UID)))
		fmt.Fprintf(&b, `<Email:DateReceived xmlns:Email="Email:">%s</Email:DateReceived>`, m.InternalDate.UTC().Format("2006-01-02T15:04:05.000Z"))
		fmt.Fprintf(&b, `<Email:Read xmlns:Email="Email:">%d</Email:Read>`, bool01(storage.HasFlag(m.Flags, `\Seen`)))
		b.WriteString(`</ApplicationData></Add>`)
	}
	b.WriteString(`</Commands></Collection></Collections></Sync>`)
	return b.String(), nil
}

func (h *easHandler) provision(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice) (string, error) {
	_ = u
	policy := storage.NewID() // UUID policy key
	if err := h.store.SetFlowSyncPolicyKey(ctx, dev.ID, policy); err != nil {
		return "", err
	}
	return `<?xml version="1.0" encoding="utf-8"?>` +
		`<Provision xmlns="Provision:"><Status>1</Status>` +
		`<Policies><Policy><PolicyType>MS-EAS-Provisioning-WBXML</PolicyType>` +
		`<Status>1</Status><PolicyKey>` + xmlEscape(policy) + `</PolicyKey>` +
		`</Policy></Policies></Provision>`, nil
}

func (h *easHandler) itemEstimate(ctx context.Context, u *storage.User) (string, error) {
	mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		return "", err
	}
	msgs, err := h.store.ListMessages(ctx, mb.ID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><GetItemEstimate xmlns="GetItemEstimate:"><Response><Status>1</Status><Collection><CollectionId>%s</CollectionId><Estimate>%d</Estimate></Collection></Response></GetItemEstimate>`,
		xmlEscape(mb.ID), len(msgs)), nil
}

func (h *easHandler) mailboxByID(ctx context.Context, userID, id string) (*storage.Mailbox, error) {
	mbs, err := h.store.ListMailboxes(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, mb := range mbs {
		if mb.ID == id {
			return mb, nil
		}
	}
	return nil, storage.ErrNotFound
}

func folderType(name string) int {
	switch strings.ToUpper(name) {
	case "INBOX":
		return 2
	case "DRAFTS":
		return 3
	case "DELETED ITEMS", "TRASH":
		return 4
	case "SENT", "SENT ITEMS":
		return 5
	case "OUTBOX":
		return 6
	default:
		return 12
	}
}

func nextSyncKey(cur string) string {
	if cur == "" || cur == "0" {
		return "1"
	}
	n, err := strconv.Atoi(cur)
	if err != nil {
		return "1"
	}
	return strconv.Itoa(n + 1)
}

func bool01(v bool) int {
	if v {
		return 1
	}
	return 0
}

func extractTag(body, local string) string {
	// Match </ns:Local> or </Local> closing after content.
	lower := body
	candidates := []string{"<" + local + ">", ":" + local + ">"}
	var start int = -1
	var openLen int
	for _, c := range candidates {
		if i := strings.Index(lower, c); i >= 0 {
			start = i + len(c)
			openLen = i
			_ = openLen
			break
		}
	}
	if start < 0 {
		return ""
	}
	rest := body[start:]
	endMarkers := []string{"</" + local + ">", "</"}
	for _, em := range endMarkers {
		if j := strings.Index(rest, em); j >= 0 {
			// For generic "</" find matching local name
			if em == "</" {
				closeIdx := strings.Index(rest, "</")
				if closeIdx < 0 {
					continue
				}
				return strings.TrimSpace(rest[:closeIdx])
			}
			return strings.TrimSpace(rest[:j])
		}
	}
	return ""
}
