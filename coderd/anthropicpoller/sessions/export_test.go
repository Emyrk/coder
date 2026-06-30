package sessions

import (
	"context"

	"github.com/google/uuid"
)

// ExportedAPIKeyForUser exposes the unexported apiKeyForUser method
// to the external sessions_test package without widening the public
// API. Standard Go _test.go export-shim idiom.
func ExportedAPIKeyForUser(ctx context.Context, s *Service, userID uuid.UUID) (string, error) {
	return s.apiKeyForUser(ctx, userID)
}

// ExportedStampReservedMetadata exposes stampReservedMetadata for
// table-driven tests of the reserved-key overwrite behavior.
func ExportedStampReservedMetadata(in map[string]string, coderUserID string) map[string]string {
	return stampReservedMetadata(in, coderUserID)
}
