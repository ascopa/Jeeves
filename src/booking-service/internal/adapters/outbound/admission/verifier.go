package admission

import (
	"jeeves/booking-service/internal/core/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const Issuer = "jeeves-waiting-room"

type claims struct {
	EventID string `json:"eid"`
	jwt.RegisteredClaims
}

type Verifier struct {
	secret []byte
	parser *jwt.Parser
}

func NewVerifier(secret []byte) *Verifier {
	return &Verifier{
		secret: secret,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
			jwt.WithIssuer(Issuer),
		),
	}
}

func (v *Verifier) Verify(token string, userID, eventID uuid.UUID) error {
	if token == "" {
		return domain.ErrAdmissionRequired
	}
	var c claims
	if _, err := v.parser.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return v.secret, nil }); err != nil {
		return domain.ErrAdmissionRequired
	}
	if c.Subject != userID.String() || c.EventID != eventID.String() {
		return domain.ErrAdmissionRequired
	}
	return nil
}
