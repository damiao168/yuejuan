package grading

import (
	"encoding/json"
	"fmt"
)

// decodeJSONB decodes a jsonb column value scanned from Postgres into dst.
// Empty input is treated as "no value" and leaves dst untouched, preserving
// the semantics of the previous len(raw) > 0 guards. These jsonb columns are
// written exclusively by this application, so a decode failure means the
// stored structure has drifted from the Go types; it must surface as an
// explicit error instead of silently zeroing the destination.
func decodeJSONB(raw []byte, dst any, field string) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode %s: %w", field, err)
	}
	return nil
}

// decodeJSONBLenient decodes a jsonb column that has no write-side schema
// guarantee, leaving dst zero-valued when the payload does not fit. Reserve it
// for presentational fields whose absence degrades a single row: failing the
// whole query would let one malformed record take down an entire exam's
// results listing.
func decodeJSONBLenient(raw []byte, dst any) {
	if len(raw) == 0 {
		return
	}
	_ = json.Unmarshal(raw, dst)
}
