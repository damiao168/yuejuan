package review

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
