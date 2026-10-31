package outbound

import "github.com/google/uuid"

type AdmissionVerifier interface {
	Verify(token string, userID, eventID uuid.UUID) error
}
