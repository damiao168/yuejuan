package modelgovernance

import (
	"errors"
	"testing"
)

func TestEnvironmentSecretResolverProbesWithoutReturningSecret(t *testing.T) {
	t.Setenv("STORY061_VENDOR_KEY", "super-secret-value")
	resolver := NewEnvironmentSecretResolver(t.TempDir())
	probe, err := resolver.Probe("env://STORY061_VENDOR_KEY")
	if err != nil {
		t.Fatalf("probe configured environment reference: %v", err)
	}
	if probe.Scheme != "env" || !probe.ResolverSupported || !probe.Configured {
		t.Fatalf("unexpected probe: %#v", probe)
	}
}

func TestEnvironmentSecretResolverRejectsTraversalAndPlaintext(t *testing.T) {
	resolver := NewEnvironmentSecretResolver(t.TempDir())
	for _, reference := range []string{
		"actual-secret-value",
		"docker_secret://../secret",
		"env://folder/secret",
	} {
		if _, err := resolver.Probe(reference); !errors.Is(err, ErrInvalidSecretReference) {
			t.Fatalf("expected invalid reference for %q, got %v", reference, err)
		}
	}
}

func TestEnvironmentSecretResolverDoesNotClaimUnconfiguredManagers(t *testing.T) {
	resolver := NewEnvironmentSecretResolver(t.TempDir())
	probe, err := resolver.Probe("vault://edugrade/vendor-a")
	if err != nil {
		t.Fatalf("probe valid future resolver reference: %v", err)
	}
	if probe.Scheme != "vault" || probe.ResolverSupported || probe.Configured {
		t.Fatalf("unsupported resolver must fail closed: %#v", probe)
	}
}
