// SPDX-License-Identifier: Apache-2.0
// Copyright 2024-2026 Кислов Роман Сергеевич.

package flowsync

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/tayga/tms/internal/mailstore"
	"github.com/tayga/tms/internal/storage"
)

// ActiveSync-compatible endpoint handled by original FlowSync engine.
type easHandler struct {
	sender *mailSubmission
	syncMu sync.Mutex
	store  storage.Driver
	ms     *mailstore.Store
}

func (h *easHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("MS-Server-ActiveSync", "18.0")
	w.Header().Set("MS-ASProtocolVersions", "14.0,14.1,16.0,16.1")
	w.Header().Set("MS-ASProtocolCommands", "FolderSync,FolderCreate,FolderDelete,FolderUpdate,Sync,MoveItems,Ping,Provision,GetItemEstimate,ItemOperations,SendMail,Settings")
	w.Header().Set("X-FlowSync", "Tayga-FlowSync")
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
	if len(deviceID) > 256 || len(deviceType) > 100 {
		http.Error(w, "invalid device", 400)
		return
	}
	dev, err := h.store.EnsureFlowSyncDevice(r.Context(), u.ID, deviceID, deviceType)
	if err != nil {
		http.Error(w, "device error", http.StatusInternalServerError)
		return
	}

	version := r.Header.Get("MS-ASProtocolVersion")
	if err = h.store.TouchFlowSyncDevice(r.Context(), dev.ID, r.RemoteAddr, r.UserAgent(), version, false); err != nil {
		http.Error(w, "device update failed", 500)
		return
	}
	if dev.Blocked && !(strings.EqualFold(cmd, "provision") && (dev.WipeStatus == "pending" || dev.WipeStatus == "sent")) {
		http.Error(w, "device blocked", 403)
		return
	}
	if (dev.WipeStatus == "pending" || dev.WipeStatus == "sent") && !strings.EqualFold(cmd, "provision") {
		w.WriteHeader(449)
		return
	}
	body, readErr := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
	if readErr != nil || len(body) > 8<<20 {
		http.Error(w, "invalid or oversized request", http.StatusBadRequest)
		return
	}
	useWBXML := requestWantsWBXML(r.Header.Get("Content-Type"), r.Header.Get("Accept"), body)
	if len(body) > 0 && body[0] == wbxmlVersion {
		body, err = decodeWire(body)
		if err != nil {
			http.Error(w, "invalid WBXML request", 400)
			return
		}
	}
	if len(body) > 0 {
		if err = validateCommandXML(body, cmd); err != nil {
			http.Error(w, "invalid command document", 400)
			return
		}
	}

	var (
		xmlOut string
		binOut []byte
	)
	switch strings.ToLower(cmd) {
	case "sendmail":
		xmlOut, err = h.sendMail(r.Context(), u, body)
	case "itemoperations":
		xmlOut, err = h.itemOperations(r.Context(), u, body)
	case "settings":
		xmlOut, err = settingsResponse(u, body)
	case "foldersync":
		xmlOut, err = h.hierarchySync(r.Context(), u, dev, body)
	case "foldercreate":
		xmlOut, err = h.mutateHierarchy(r.Context(), u, dev, "FolderCreate", body, h.folderCreate)
	case "folderdelete":
		xmlOut, err = h.mutateHierarchy(r.Context(), u, dev, "FolderDelete", body, h.folderDelete)
	case "folderupdate":
		xmlOut, err = h.mutateHierarchy(r.Context(), u, dev, "FolderUpdate", body, h.folderUpdate)
	case "sync":
		xmlOut, err = h.syncCollections(r.Context(), u, dev, body)
	case "moveitems":
		xmlOut, binOut, err = h.moveItems(r.Context(), u, body, false)
	case "provision":
		xmlOut, binOut, err = h.provision(r.Context(), u, dev, false, body)
	case "ping":
		xmlOut, err = h.ping(r.Context(), u, dev, body)
	case "getitemestimate":
		xmlOut, binOut, err = h.itemEstimate(r.Context(), u, dev, body, false)
	default:
		http.Error(w, "unsupported command", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "FlowSync command failed", http.StatusInternalServerError)
		return
	}
	if useWBXML && xmlOut != "" {
		binOut, err = encodeWire(xmlOut)
		if err != nil {
			http.Error(w, "response encoding failed", 500)
			return
		}
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
	if err := (&ewsHandler{store: h.store, ms: h.ms}).ensureStandardMailFolders(ctx, u); err != nil {
		return "", nil, err
	}
	_ = h.store.EnsureDAVDefaults(ctx, u.ID)
	_ = h.store.EnsureNoteDefaults(ctx, u.ID)

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
	nfs, err := h.store.ListNoteFoldersForUser(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}

	next := "snapshot"

	folders := make([]folderChange, 0, len(mbs)+len(cals)+len(abs)+len(nfs))
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
			ServerID: cal.ID, ParentID: "0", DisplayName: name, Type: defaultFolderType(cal.Name, folderTypeCalendar, 13),
		})
	}
	for _, ab := range abs {
		name := ab.DisplayName
		if name == "" {
			name = ab.Name
		}
		folders = append(folders, folderChange{
			ServerID: ab.ID, ParentID: "0", DisplayName: name, Type: defaultFolderType(ab.Name, folderTypeContacts, 14),
		})
	}
	for _, nf := range nfs {
		name := nf.DisplayName
		if name == "" {
			name = nf.Name
		}
		folders = append(folders, folderChange{
			ServerID: nf.ID, ParentID: "0", DisplayName: name, Type: noteFolderType(nf.Name),
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

func (h *easHandler) provision(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, wbxml bool, body []byte) (string, []byte, error) {
	_ = u
	if dev.WipeStatus == "pending" || dev.WipeStatus == "sent" {
		if dev.Version != "16.1" {
			return "", nil, fmt.Errorf("account wipe requires protocol 16.1")
		}
		acknowledged := dev.WipeStatus == "sent" && accountWipeAcknowledged(body, wbxml)
		state := "sent"
		if acknowledged {
			state = "acknowledged"
		}
		if err := h.store.SetFlowSyncDeviceControl(ctx, dev.ID, dev.Blocked || acknowledged, state); err != nil {
			return "", nil, err
		}
		if wbxml {
			return "", encodeAccountWipeWBXML(acknowledged), nil
		}
		command := "<AccountOnlyRemoteWipe/>"
		if acknowledged {
			command = ""
		}
		return `<Provision xmlns="Provision:"><Status>1</Status>` + command + `</Provision>`, nil, nil
	}
	if len(body) > 0 {
		doc, err := parseProtocolXML(body)
		if err != nil {
			return "", nil, err
		}
		if p := doc.find("Policy"); p != nil && p.child("Status") != nil {
			if p.value("PolicyKey") != dev.PolicyKey || p.value("Status") != "1" || dev.PolicyKey == "" {
				return `<Provision xmlns="Provision:"><Status>1</Status><Policies><Policy><Status>5</Status></Policy></Policies></Provision>`, nil, nil
			}
			if err = h.store.TouchFlowSyncDevice(ctx, dev.ID, dev.Address, dev.Agent, dev.Version, true); err != nil {
				return "", nil, err
			}
			return fmt.Sprintf(`<Provision xmlns="Provision:"><Status>1</Status><Policies><Policy><PolicyType>MS-EAS-Provisioning-WBXML</PolicyType><Status>1</Status><PolicyKey>%s</PolicyKey></Policy></Policies></Provision>`, xmlEscape(dev.PolicyKey)), nil, nil
		}
	}
	policy := dev.PolicyKey
	if value, parseErr := strconv.ParseUint(policy, 10, 32); parseErr != nil || value == 0 {
		var random [4]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		value := binary.BigEndian.Uint32(random[:])
		if value == 0 {
			value = 1
		}
		policy = strconv.FormatUint(uint64(value), 10)
	}
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
	b.WriteString(`<Data><EASProvisionDoc>`)
	fmt.Fprintf(&b, `<DevicePasswordEnabled>%d</DevicePasswordEnabled>`, bool01(p.DevicePasswordEnabled))
	fmt.Fprintf(&b, `<MinDevicePasswordLength>%d</MinDevicePasswordLength>`, p.MinDevicePasswordLength)
	fmt.Fprintf(&b, `<MaxInactivityTimeDeviceLock>%d</MaxInactivityTimeDeviceLock>`, p.MaxInactivityTimeDeviceLock)
	fmt.Fprintf(&b, `<MaxDevicePasswordFailedAttempts>%d</MaxDevicePasswordFailedAttempts>`, p.MaxDevicePasswordFailedAttempts)
	fmt.Fprintf(&b, `<AllowSimpleDevicePassword>%d</AllowSimpleDevicePassword>`, bool01(p.AllowSimpleDevicePassword))
	fmt.Fprintf(&b, `<AlphanumericDevicePasswordRequired>%d</AlphanumericDevicePasswordRequired>`, bool01(p.AlphanumericDevicePasswordRequired))
	fmt.Fprintf(&b, `<RequireDeviceEncryption>%d</RequireDeviceEncryption>`, bool01(p.RequireDeviceEncryption))
	fmt.Fprintf(&b, `<AllowStorageCard>%d</AllowStorageCard>`, bool01(p.AllowStorageCard))
	fmt.Fprintf(&b, `<AllowCamera>%d</AllowCamera>`, bool01(p.AllowCamera))
	b.WriteString(`</EASProvisionDoc></Data></Policy></Policies></Provision>`)
	return b.String(), nil, nil
}

func (h *easHandler) itemEstimate(ctx context.Context, u *storage.User, dev *storage.FlowSyncDevice, reqBody []byte, wbxml bool) (string, []byte, error) {
	doc, err := parseProtocolXML(reqBody)
	if err != nil {
		return "", nil, err
	}
	collections := doc.find("Collections")
	if collections == nil || len(collections.Children) == 0 || len(collections.Children) > 100 {
		return `<GetItemEstimate xmlns="GetItemEstimate:"><Response><Status>2</Status></Response></GetItemEstimate>`, nil, nil
	}
	var out strings.Builder
	out.WriteString(`<GetItemEstimate xmlns="GetItemEstimate:">`)
	for _, request := range collections.Children {
		id := request.value("CollectionId")
		kind, e := h.resolveCollection(ctx, u.ID, id)
		status, count := 1, 0
		if e != nil {
			status = 2
		} else {
			state, e := loadCollectionState(ctx, h.store, u.ID, dev.ID, id)
			if e != nil {
				return "", nil, e
			}
			key := request.value("SyncKey")
			if key != "0" && key != state.Key {
				status = 4
			} else {
				prefs, e := preferencesFrom(request, state.Preferences)
				if e != nil || !validFilter(kind, prefs) {
					status = 2
				} else {
					snapshotCtx := context.WithValue(ctx, preferencesKey{}, prefs)
					var full string
					switch kind {
					case kindCalendar:
						full, _, e = h.syncCalendar(snapshotCtx, id, "unused", false, nil)
					case kindContacts:
						full, _, e = h.syncContacts(snapshotCtx, id, "unused", false, nil)
					case kindNotes:
						full, _, e = h.syncNotes(snapshotCtx, u, id, "unused", false, nil)
					default:
						full, _, e = h.syncMailCollection(context.WithValue(snapshotCtx, mailSnapshotKey{}, true), u, id, "unused", false, nil)
					}
					if e != nil {
						return "", nil, e
					}
					parsed, e := parseProtocolXML([]byte(full))
					if e != nil {
						return "", nil, e
					}
					hashes := map[string]string{}
					if commands := parsed.find("Commands"); commands != nil {
						for _, add := range commands.Children {
							hashes[add.value("ServerId")] = digest(add.render() + prefs.hash())
						}
					}
					previous := state.Items
					if key == "0" {
						previous = nil
					}
					changes, _, _ := diffItems(previous, hashes, len(previous)+len(hashes)+1)
					count = len(changes)
				}
			}
		}
		fmt.Fprintf(&out, `<Response><Status>%d</Status><Collection><CollectionId>%s</CollectionId><Estimate>%d</Estimate></Collection></Response>`, status, xmlEscape(id), count)
	}
	out.WriteString(`</GetItemEstimate>`)
	if wbxml {
		raw, e := encodeWire(out.String())
		return "", raw, e
	}
	return out.String(), nil, nil
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
	decoder := xml.NewDecoder(strings.NewReader(body))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == local {
			var value string
			if decoder.DecodeElement(&value, &start) != nil {
				return ""
			}
			return strings.TrimSpace(value)
		}
	}
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

func defaultFolderType(name string, standard, custom int) int {
	if name == "default" {
		return standard
	}
	return custom
}

func noteFolderType(name string) int {
	if name == "notes" {
		return folderTypeNotes
	}
	return 17
}
