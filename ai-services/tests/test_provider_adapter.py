import unittest
from contextlib import contextmanager

from grading_agent.app import GradingAgentApplication
from grading_agent.model import LocalLlamaCppAdapter
from grading_agent.provider_adapter import (
    ProviderAdapter,
    ProviderAdapterRegistry,
    build_provider_adapter,
    default_provider_adapter_registry,
)
from helpers import settings


class CompleteAdapter:
    @contextmanager
    def session(self, _request_id):
        yield

    def request(self, _grading_request, repair_reason=None):
        return {"repair_reason": repair_reason}

    def ready(self):
        return True


class IncompleteAdapter:
    def ready(self):
        return True


class ProviderAdapterRegistryTests(unittest.TestCase):
    def test_default_build_enables_only_local_adapter(self):
        registry = default_provider_adapter_registry()
        self.assertEqual(registry.enabled_types(), ("local_llama_cpp",))
        adapter = build_provider_adapter(settings(), registry)
        self.assertIsInstance(adapter, LocalLlamaCppAdapter)
        self.assertIsInstance(adapter, ProviderAdapter)

    def test_registry_selects_exact_registered_adapter(self):
        registry = ProviderAdapterRegistry()
        adapter = CompleteAdapter()
        registry.register("fixture_native", lambda _settings: adapter)
        created = registry.create(settings(adapter_type="fixture_native"))
        self.assertIs(created, adapter)

    def test_application_builds_model_from_explicit_registry(self):
        registry = ProviderAdapterRegistry()
        adapter = CompleteAdapter()
        registry.register("fixture_native", lambda _settings: adapter)
        app = GradingAgentApplication(
            settings(adapter_type="fixture_native"),
            adapter_registry=registry,
        )
        self.assertIs(app.model, adapter)

    def test_unknown_adapter_fails_closed(self):
        registry = ProviderAdapterRegistry()
        with self.assertRaisesRegex(ValueError, "not enabled"):
            registry.create(settings(adapter_type="unregistered_native"))

    def test_duplicate_adapter_registration_is_rejected(self):
        registry = ProviderAdapterRegistry()
        registry.register("fixture_native", lambda _settings: CompleteAdapter())
        with self.assertRaisesRegex(ValueError, "already registered"):
            registry.register("fixture_native", lambda _settings: CompleteAdapter())

    def test_incomplete_adapter_is_rejected(self):
        registry = ProviderAdapterRegistry()
        registry.register("fixture_native", lambda _settings: IncompleteAdapter())
        with self.assertRaisesRegex(TypeError, "does not implement"):
            registry.create(settings(adapter_type="fixture_native"))


if __name__ == "__main__":
    unittest.main()
