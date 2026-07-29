package modelgovernance

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidSecretReference = errors.New("invalid secret reference")

type SecretProbe struct {
	Scheme            string `json:"scheme"`
	ResolverSupported bool   `json:"resolver_supported"`
	Configured        bool   `json:"configured"`
}

type SecretReferenceResolver interface {
	Probe(reference string) (SecretProbe, error)
}

type EnvironmentSecretResolver struct {
	dockerSecretDir string
}

func NewEnvironmentSecretResolver(dockerSecretDir string) *EnvironmentSecretResolver {
	if strings.TrimSpace(dockerSecretDir) == "" {
		dockerSecretDir = "/run/secrets"
	}
	return &EnvironmentSecretResolver{dockerSecretDir: dockerSecretDir}
}

func (r *EnvironmentSecretResolver) Probe(reference string) (SecretProbe, error) {
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return SecretProbe{}, ErrInvalidSecretReference
	}
	name := strings.TrimPrefix(parsed.Host+parsed.Path, "/")
	if name == "" || strings.Contains(name, "..") {
		return SecretProbe{}, ErrInvalidSecretReference
	}
	switch parsed.Scheme {
	case "env":
		if strings.ContainsAny(name, `/\`) {
			return SecretProbe{}, ErrInvalidSecretReference
		}
		value, exists := os.LookupEnv(name)
		return SecretProbe{Scheme: parsed.Scheme, ResolverSupported: true, Configured: exists && value != ""}, nil
	case "docker_secret":
		if strings.ContainsAny(name, `/\`) {
			return SecretProbe{}, ErrInvalidSecretReference
		}
		info, statErr := os.Stat(filepath.Join(r.dockerSecretDir, name))
		return SecretProbe{
			Scheme:            parsed.Scheme,
			ResolverSupported: true,
			Configured:        statErr == nil && !info.IsDir() && info.Size() > 0,
		}, nil
	case "vault", "aws_secrets_manager", "azure_key_vault", "gcp_secret_manager":
		return SecretProbe{Scheme: parsed.Scheme, ResolverSupported: false, Configured: false}, nil
	default:
		return SecretProbe{}, ErrInvalidSecretReference
	}
}
