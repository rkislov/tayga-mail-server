package smtp

import (
	"bytes"
	"errors"

	"github.com/emersion/go-sasl"
)

// loginAuthenticator verifies username/password for the obsolete LOGIN mechanism.
type loginAuthenticator func(username, password string) error

type loginServer struct {
	authenticate loginAuthenticator
	username     string
	step         int
}

func newLoginServer(auth loginAuthenticator) sasl.Server {
	return &loginServer{authenticate: auth}
}

func (s *loginServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if s.authenticate == nil {
		return nil, false, errors.New("sasl login: missing authenticator")
	}
	switch s.step {
	case 0:
		s.step++
		if len(response) == 0 {
			return []byte("Username:"), false, nil
		}
		s.username = string(response)
		return []byte("Password:"), false, nil
	case 1:
		s.step++
		if err := s.authenticate(s.username, string(response)); err != nil {
			return nil, false, err
		}
		return nil, true, nil
	default:
		if !bytes.Equal(response, []byte{}) {
			return nil, false, sasl.ErrUnexpectedClientResponse
		}
		return nil, true, nil
	}
}
