package mattermost

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tayga/tms/internal/chat"
)

func (s *API) pushKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	s.pushSignMu.Lock()
	defer s.pushSignMu.Unlock()
	if s.pushSigningKey != nil {
		return s.pushSigningKey, nil
	}
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return nil, e
	}
	der, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return nil, e
	}
	encoded, e := s.chat.InitSetting(ctx, "mattermost.push.signing_private_key", base64.StdEncoding.EncodeToString(der))
	if e != nil {
		return nil, e
	}
	der, e = base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return nil, fmt.Errorf("invalid push signing key encoding")
	}
	parsed, e := x509.ParsePKCS8PrivateKey(der)
	if e != nil {
		return nil, fmt.Errorf("invalid push signing key")
	}
	stored, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || stored.Curve != elliptic.P256() {
		return nil, fmt.Errorf("push signing key must be P-256")
	}
	s.pushSigningKey = stored
	return stored, nil
}
func (s *API) pushPublicKey(ctx context.Context) (string, error) {
	key, e := s.pushKey(ctx)
	if e != nil {
		return "", e
	}
	der, e := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if e != nil {
		return "", e
	}
	return base64.StdEncoding.EncodeToString(der), nil
}
func (s *API) signPushPayload(ctx context.Context, payload map[string]any, ackID, deviceToken string) error {
	key, e := s.pushKey(ctx)
	if e != nil {
		return e
	}
	signature, e := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"ack_id": ackID, "device_id": deviceToken}).SignedString(key)
	if e != nil {
		return e
	}
	payload["signature"], payload["ack_id"] = signature, ackID
	return nil
}

var pushAckID = regexp.MustCompile(`^[a-z2-7]{26}$`)

// The opaque receipt ID permits acknowledgement only. No session, chat content,
// signing key or device metadata is returned by this pre-authentication route.
func (s *API) pushReceiptHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		PostID string `json:"post_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !pushAckID.MatchString(body.ID) || (body.PostID != "" && !pushAckID.MatchString(body.PostID)) {
		s.failure(w, chat.ErrInvalid)
		return
	}
	changed, e := s.chat.AckPushReceipt(r.Context(), body.ID, body.PostID)
	if e != nil {
		s.failure(w, e)
		return
	}
	if changed {
		s.log.Info("push client receipt received", "ack_id", body.ID)
	}
	write(w, 200, map[string]string{"status": "OK"})
}
