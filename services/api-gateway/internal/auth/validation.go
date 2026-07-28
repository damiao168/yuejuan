package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	maxTenantCodeBytes   = 128
	maxUsernameBytes     = 256
	maxDisplayNameBytes  = 256
	maxRoleCodeBytes     = 128
	maxPasswordBytes     = 72
	maxAuthJSONBodyBytes = 4 * 1024
)

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, target any, disallowUnknownFields bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if disallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func authRequestBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func loginFieldsWithinLimits(tenantCode string, username string, password string) bool {
	return len(tenantCode) <= maxTenantCodeBytes &&
		len(username) <= maxUsernameBytes &&
		len(password) <= maxPasswordBytes
}

func managedUserFieldsWithinLimits(input CreateManagedUserInput) bool {
	return len(input.Username) <= maxUsernameBytes &&
		len(input.DisplayName) <= maxDisplayNameBytes &&
		len(input.RoleCode) <= maxRoleCodeBytes &&
		len(input.Password) <= maxPasswordBytes
}

func bootstrapFieldsWithinLimits(input BootstrapAdminInput) bool {
	return len(input.TenantCode) <= maxTenantCodeBytes &&
		len(input.Username) <= maxUsernameBytes &&
		len(input.DisplayName) <= maxDisplayNameBytes &&
		len(input.RoleCode) <= maxRoleCodeBytes &&
		len(input.Password) <= maxPasswordBytes
}
