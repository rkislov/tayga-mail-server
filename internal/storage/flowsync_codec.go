package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

const syncStateEnvelopePrefix = "tms-sync-v1:"
const maxCompactState = 64 << 20
const syncStateColumns = "s.state,s.snapshot_id,b.data,b.raw_bytes,s.response_id,r.data,r.raw_bytes"
const syncStateFrom = " FROM flowsync_states s LEFT JOIN flowsync_state_blobs b ON b.id=s.snapshot_id AND b.user_id=s.user_id AND b.collection_id=s.collection_id LEFT JOIN flowsync_state_blobs r ON r.id=s.response_id AND r.user_id=s.user_id AND r.collection_id=s.collection_id"

type syncEnvelope struct {
	Before string   `json:"before,omitempty"`
	Middle string   `json:"middle,omitempty"`
	After  string   `json:"after,omitempty"`
	Whole  string   `json:"whole,omitempty"`
	Keys   []string `json:"keys,omitempty"`
}
type compactState struct {
	state, id, responseID   string
	data, responseData      []byte
	rawBytes, responseBytes int
}
type syncStateBlob struct {
	id   string
	data []byte
	size int64
}
type storedSyncState struct {
	state                  string
	itemID, responseID     sql.NullString
	itemData, responseData []byte
	itemSize, responseSize sql.NullInt64
}

func (s *storedSyncState) targets() []any {
	return []any{&s.state, &s.itemID, &s.itemData, &s.itemSize, &s.responseID, &s.responseData, &s.responseSize}
}
func (s storedSyncState) decode(user, collection string) (string, error) {
	return decodeSyncState(user, collection, s.state, syncStateBlob{s.itemID.String, s.itemData, s.itemSize.Int64}, syncStateBlob{s.responseID.String, s.responseData, s.responseSize.Int64})
}
func syncBlobID(user, collection string, value []byte) string {
	digest := sha256.New()
	_, _ = io.WriteString(digest, user+"\x00"+collection+"\x00")
	_, _ = digest.Write(value)
	return hex.EncodeToString(digest.Sum(nil))
}
func syncKeyMarker(i int) []byte { return []byte("\x00TMSKEY" + strconv.Itoa(i) + "\x00") }

// Items and cached replay XML are shared independently. Opaque per-client keys
// in replay XML become reversible placeholders, never discarded or regenerated.
func encodeSyncState(user, client, collection, raw string) (compactState, error) {
	out := compactState{state: raw}
	if len(raw) < 4096 || len(raw) > maxCompactState || !json.Valid([]byte(raw)) {
		return out, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return out, nil
	}
	itemsStart, itemsEnd, responseStart, responseEnd := -1, 0, -1, 0
	keys := []string{client}
	for decoder.More() {
		key, e := decoder.Token()
		if e != nil {
			return out, e
		}
		var value json.RawMessage
		if e = decoder.Decode(&value); e != nil {
			return out, e
		}
		end := int(decoder.InputOffset())
		start := end - len(value)
		if key == "Items" && len(value) >= 1024 && value[0] == '{' {
			itemsStart, itemsEnd = start, end
		}
		if key == "Response" && len(value) >= 1024 && value[0] == '"' {
			responseStart, responseEnd = start, end
		}
		if key == "Key" || key == "PreviousKey" {
			var text string
			if json.Unmarshal(value, &text) == nil {
				keys = append(keys, text)
			}
		}
	}
	envelope := syncEnvelope{Whole: raw}
	if itemsStart >= 0 {
		value := []byte(raw[itemsStart:itemsEnd])
		out.id = syncBlobID(user, collection, value)
		out.rawBytes = len(value)
		out.data, err = gzipSyncBytes(value)
		if err != nil {
			return out, err
		}
		envelope = syncEnvelope{Before: raw[:itemsStart], After: raw[itemsEnd:]}
	}
	// Unusual legacy field orders remain losslessly compressed without factoring
	// their Response. The normal struct/map order puts Response after Items.
	if responseStart >= 0 && (itemsStart < 0 || responseStart >= itemsEnd) {
		value := []byte(raw[responseStart:responseEnd])
		unique := []string{}
		for _, key := range keys {
			if len(key) < 16 || len(key) > 256 || strings.ContainsRune(key, 0) {
				continue
			}
			found := false
			for _, v := range unique {
				if v == key {
					found = true
				}
			}
			if !found {
				unique = append(unique, key)
			}
		}
		for i, key := range unique {
			value = bytes.ReplaceAll(value, []byte(key), syncKeyMarker(i))
		}
		out.responseID = syncBlobID(user, collection, value)
		out.responseBytes = len(value)
		out.responseData, err = gzipSyncBytes(value)
		if err != nil {
			return out, err
		}
		envelope.Keys = unique
		envelope.Whole = ""
		if itemsStart >= 0 {
			envelope.Middle = raw[itemsEnd:responseStart]
		} else {
			envelope.Before = raw[:responseStart]
		}
		envelope.After = raw[responseEnd:]
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return out, err
	}
	if len(data) > maxCompactState {
		return compactState{state: raw}, nil
	}
	packed, err := gzipSyncBytes(data)
	if err != nil {
		return out, err
	}
	encoded := syncStateEnvelopePrefix + base64.StdEncoding.EncodeToString(packed)
	if out.id == "" && out.responseID == "" && len(encoded) >= len(raw)*8/10 {
		return compactState{state: raw}, nil
	}
	out.state = encoded
	return out, nil
}
func gzipSyncBytes(raw []byte) ([]byte, error) {
	var out bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&out, gzip.BestSpeed)
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func unpackSyncBytes(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("invalid compressed synchronization state")
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, maxCompactState+1))
	if err != nil || len(raw) > maxCompactState {
		return nil, errors.New("invalid synchronization state size")
	}
	return raw, nil
}
func decodeSyncBlob(user, collection string, blob syncStateBlob) ([]byte, error) {
	if blob.id == "" {
		return nil, nil
	}
	raw, err := unpackSyncBytes(blob.data)
	if err != nil || int64(len(raw)) != blob.size || syncBlobID(user, collection, raw) != blob.id {
		return nil, errors.New("invalid synchronization snapshot scope or digest")
	}
	return raw, nil
}
func decodeSyncState(user, collection, state string, items, response syncStateBlob) (string, error) {
	if !strings.HasPrefix(state, syncStateEnvelopePrefix) {
		if items.id != "" || response.id != "" {
			return "", errors.New("unexpected snapshot reference")
		}
		return state, nil
	}
	packed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(state, syncStateEnvelopePrefix))
	if err != nil {
		return "", errors.New("invalid synchronization envelope")
	}
	raw, err := unpackSyncBytes(packed)
	if err != nil {
		return "", err
	}
	var envelope syncEnvelope
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Keys) > 3 {
		return "", errors.New("invalid synchronization envelope")
	}
	if items.id == "" && response.id == "" {
		if envelope.Whole == "" || !json.Valid([]byte(envelope.Whole)) {
			return "", errors.New("missing synchronization state")
		}
		return envelope.Whole, nil
	}
	itemValue, err := decodeSyncBlob(user, collection, items)
	if err != nil {
		return "", err
	}
	responseValue, err := decodeSyncBlob(user, collection, response)
	if err != nil {
		return "", err
	}
	for i, key := range envelope.Keys {
		if len(key) > 256 || strings.ContainsRune(key, 0) {
			return "", errors.New("invalid synchronization key")
		}
		responseValue = bytes.ReplaceAll(responseValue, syncKeyMarker(i), []byte(key))
	}
	value := envelope.Before + string(itemValue)
	if response.id != "" {
		value += envelope.Middle + string(responseValue)
	}
	value += envelope.After
	if len(value) > maxCompactState || !json.Valid([]byte(value)) {
		return "", errors.New("invalid reconstructed synchronization state")
	}
	return value, nil
}
