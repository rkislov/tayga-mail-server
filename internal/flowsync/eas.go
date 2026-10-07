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
	useWBXML := requestWantsWBXML(r.Header.Get("Content-Type"), r.Header.Get("Accept"), body)

	var (
		xmlOut string
		binOut []byte
	)
	switch strings.ToLower(cmd) {
	case "foldersync":
		xmlOut, binOut, err = h.folderSync(r.Context(), u, dev, useWBXML)
	case "sync":
		xmlOut, binOut, err = h.syncCollection(r.Context(), u, dev, body, useWBXML)
	case "provision":
		xmlOut, binOut, err = h.provision(r.Context(), u, dev, useWBXML)
	case "ping":
		if useWBXML {
			binOut = encodePingWBXML()
		} else {
			xmlOut = `<?xml version="1.0" encoding="utf-8"?><Ping xmlns="Ping:"><Status>1</Status></Ping>`
		}
	case "getitemestimate":
		xmlOut, binOut, err = h.itemEstimate(r.Context(), u, body, useWBXML)
	default:
		http.Error(w, "unsupported command", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if useWBXML && binOut != nil {
		w.Header().Set("Content-Type", "application/vnd.ms-sync.wbxml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(binOut)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xmlOut))
}

func (h *easHandler) folderSync(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, wbxml bool) (string, []byte, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)

	mbs, err := h.store.ListMailboxes(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}
	cals, err := h.store.ListCalendars(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}
	abs, err := h.store.ListAddressBooks(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}

	key, err := h.store.GetFlowSyncSyncKey(ctx, dev.ID, "hierarchy")
	if err != nil {
		return "", nil, err
	}
	next := nextSyncKey(key)
	if err := h.store.SetFlowSyncSyncKey(ctx, dev.ID, "hierarchy", next); err != nil {
		return "", nil, err
	}

	folders := make([]folderChange, 0, len(mbs)+len(cals)+len(abs))
	for _, mb := range mbs {
		folders = append(folders, folderChange{
			ServerID: mb.ID, ParentID: "0", DisplayName: mb.Name, Type: folderType(mb.Name),
		})
	}
	for _, cal := range cals {
		name := cal.DisplayName
		if name == "" {
			name = cal.Name
		}
		folders = append(folders, folderChange{
			ServerID: cal.ID, ParentID: "0", DisplayName: name, Type: folderTypeCalendar,
		})
	}
	for _, ab := range abs {
		name := ab.DisplayName
		if name == "" {
			name = ab.Name
		}
		folders = append(folders, folderChange{
			ServerID: ab.ID, ParentID: "0", DisplayName: name, Type: folderTypeContacts,
		})
	}
	if wbxml {
		return "", encodeFolderSyncWBXML(next, folders), nil
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<FolderSync xmlns="FolderHierarchy:">`)
	b.WriteString(`<Status>1</Status>`)
	fmt.Fprintf(&b, `<SyncKey>%s</SyncKey>`, xmlEscape(next))
	b.WriteString(`<Changes>`)
	fmt.Fprintf(&b, `<Count>%d</Count>`, len(folders))
	for _, f := range folders {
		b.WriteString(`<Add>`)
		fmt.Fprintf(&b, `<ServerId>%s</ServerId>`, xmlEscape(f.ServerID))
		b.WriteString(`<ParentId>0</ParentId>`)
		fmt.Fprintf(&b, `<DisplayName>%s</DisplayName>`, xmlEscape(f.DisplayName))
		fmt.Fprintf(&b, `<Type>%d</Type>`, f.Type)
		b.WriteString(`</Add>`)
	}
	b.WriteString(`</Changes></FolderSync>`)
	return b.String(), nil, nil
}

func (h *easHandler) provision(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, wbxml bool) (string, []byte, error) {
	_ = u
	policy := storage.NewID() // UUID policy key
	if err := h.store.SetFlowSyncPolicyKey(ctx, dev.ID, policy); err != nil {
		return "", nil, err
	}
	p := defaultDevicePolicy()
	if wbxml {
		return "", encodeProvisionWBXML(policy, p), nil
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<Provision xmlns="Provision:"><Status>1</Status><Policies><Policy>`)
	b.WriteString(`<PolicyType>MS-EAS-Provisioning-WBXML</PolicyType><Status>1</Status>`)
	fmt.Fprintf(&b, `<PolicyKey>%s</PolicyKey>`, xmlEscape(policy))
	b.WriteString(`<Data>`)
	fmt.Fprintf(&b, `<DevicePasswordEnabled>%d</DevicePasswordEnabled>`, bool01(p.DevicePasswordEnabled))
	fmt.Fprintf(&b, `<MinDevicePasswordLength>%d</MinDevicePasswordLength>`, p.MinDevicePasswordLength)
	fmt.Fprintf(&b, `<MaxInactivityTimeDeviceLock>%d</MaxInactivityTimeDeviceLock>`, p.MaxInactivityTimeDeviceLock)
	fmt.Fprintf(&b, `<MaxDevicePasswordFailedAttempts>%d</MaxDevicePasswordFailedAttempts>`, p.MaxDevicePasswordFailedAttempts)
	fmt.Fprintf(&b, `<AllowSimpleDevicePassword>%d</AllowSimpleDevicePassword>`, bool01(p.AllowSimpleDevicePassword))
	fmt.Fprintf(&b, `<AlphanumericDevicePasswordRequired>%d</AlphanumericDevicePasswordRequired>`, bool01(p.AlphanumericDevicePasswordRequired))
	fmt.Fprintf(&b, `<RequireDeviceEncryption>%d</RequireDeviceEncryption>`, bool01(p.RequireDeviceEncryption))
	fmt.Fprintf(&b, `<AllowStorageCard>%d</AllowStorageCard>`, bool01(p.AllowStorageCard))
	fmt.Fprintf(&b, `<AllowCamera>%d</AllowCamera>`, bool01(p.AllowCamera))
	b.WriteString(`</Data></Policy></Policies></Provision>`)
	return b.String(), nil, nil
}

func (h *easHandler) itemEstimate(ctx context.Context, u *storage.User, reqBody []byte, wbxml bool) (string, []byte, error) {
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	collectionID := extractCollectionID(reqBody)
	if collectionID == "" {
		mb, err := h.store.GetMailbox(ctx, u.ID, "INBOX")
		if err != nil {
			return "", nil, err
		}
		collectionID = mb.ID
	}
	kind, err := h.resolveCollection(ctx, u.ID, collectionID)
	if err != nil {
		return "", nil, err
	}
	var n int
	switch kind {
	case kindCalendar:
		objs, err := h.store.ListCalendarObjects(ctx, collectionID)
		if err != nil {
			return "", nil, err
		}
		n = len(objs)
	case kindContacts:
		objs, err := h.store.ListAddressObjects(ctx, collectionID)
		if err != nil {
			return "", nil, err
		}
		n = len(objs)
	default:
		msgs, err := h.store.ListMessages(ctx, collectionID)
		if err != nil {
			return "", nil, err
		}
		n = len(msgs)
	}
	if wbxml {
		return "", encodeItemEstimateWBXML(collectionID, n), nil
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><GetItemEstimate xmlns="GetItemEstimate:"><Response><Status>1</Status><Collection><CollectionId>%s</CollectionId><Estimate>%d</Estimate></Collection></Response></GetItemEstimate>`,
		xmlEscape(collectionID), n), nil, nil
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

func extractCollectionID(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if body[0] == wbxmlVersion {
		return extractWBXMLTagString(body, "CollectionId")
	}
	return extractTag(string(body), "CollectionId")
}

func extractTag(body, local string) string {
	candidates := []string{"<" + local + ">", ":" + local + ">"}
	var start int = -1
	for _, c := range candidates {
		if i := strings.Index(body, c); i >= 0 {
			start = i + len(c)
			break
		}
	}
	if start < 0 {
		return ""
	}
	rest := body[start:]
	closeIdx := strings.Index(rest, "</")
	if closeIdx < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:closeIdx])
}

// devicePolicy is the FlowSync remote policy applied during Provision.
type devicePolicy struct {
	DevicePasswordEnabled              bool
	MinDevicePasswordLength            int
	MaxInactivityTimeDeviceLock        int // seconds
	MaxDevicePasswordFailedAttempts    int
	AllowSimpleDevicePassword          bool
	AlphanumericDevicePasswordRequired bool
	RequireDeviceEncryption            bool
	AllowStorageCard                   bool
	AllowCamera                        bool
}

func defaultDevicePolicy() devicePolicy {
	return devicePolicy{
		DevicePasswordEnabled:              true,
		MinDevicePasswordLength:            4,
		MaxInactivityTimeDeviceLock:        900,
		MaxDevicePasswordFailedAttempts:    10,
		AllowSimpleDevicePassword:          true,
		AlphanumericDevicePasswordRequired: false,
		RequireDeviceEncryption:            false,
		AllowStorageCard:                   true,
		AllowCamera:                        true,
	}
}
