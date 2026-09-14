package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	argon2Memory      = 19 * 1024
	argon2Iterations  = 2
	argon2Parallelism = 1
	argon2SaltLength  = 16
	argon2KeyLength   = 32
)

type argon2Parameters struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
}

// HashPassword writes new credentials using Argon2id in the standard PHC
// format. Existing bcrypt hashes remain readable through CheckPassword and
// are upgraded after the next successful login.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory,
		argon2Iterations,
		argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func CheckPassword(hash string, password string) bool {
	valid, _ := VerifyPassword(hash, password)
	return valid
}

// VerifyPassword also reports whether a valid legacy or outdated hash should
// be replaced. Parsing is deliberately bounded so a malformed database value
// cannot request excessive CPU or memory.
func VerifyPassword(encodedHash string, password string) (bool, bool) {
	if strings.HasPrefix(encodedHash, "$2") {
		if bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password)) != nil {
			return false, false
		}
		return true, true
	}
	parameters, err := decodeArgon2idHash(encodedHash)
	if err != nil {
		return false, false
	}
	candidate := argon2.IDKey([]byte(password), parameters.salt, parameters.iterations, parameters.memory, parameters.parallelism, uint32(len(parameters.hash)))
	valid := subtle.ConstantTimeCompare(candidate, parameters.hash) == 1
	needsRehash := valid && (parameters.memory != argon2Memory ||
		parameters.iterations != argon2Iterations ||
		parameters.parallelism != argon2Parallelism ||
		len(parameters.salt) != argon2SaltLength ||
		len(parameters.hash) != argon2KeyLength)
	return valid, needsRehash
}

func decodeArgon2idHash(encodedHash string) (argon2Parameters, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argon2Parameters{}, errors.New("invalid argon2id hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argon2Parameters{}, errors.New("unsupported argon2id version")
	}
	if parts[2] != fmt.Sprintf("v=%d", version) {
		return argon2Parameters{}, errors.New("invalid argon2id version")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return argon2Parameters{}, errors.New("invalid argon2id parameters")
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", memory, iterations, parallelism) {
		return argon2Parameters{}, errors.New("invalid argon2id parameters")
	}
	if memory < 8*1024 || memory > 128*1024 || iterations < 1 || iterations > 8 || parallelism < 1 || parallelism > 8 {
		return argon2Parameters{}, errors.New("argon2id parameters outside supported bounds")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return argon2Parameters{}, errors.New("invalid argon2id salt")
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(hash) < 16 || len(hash) > 64 {
		return argon2Parameters{}, errors.New("invalid argon2id digest")
	}
	return argon2Parameters{memory: memory, iterations: iterations, parallelism: parallelism, salt: salt, hash: hash}, nil
}

var dummyPasswordHash = func() string {
	hash, err := HashPassword("edugrade-invalid-login-comparison")
	if err != nil {
		panic("initialize password comparison: " + err.Error())
	}
	return hash
}()
