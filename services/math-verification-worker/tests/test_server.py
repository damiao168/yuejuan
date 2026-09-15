import pytest
from math_verification_worker.server import main


def test_production_requires_tls_certificate(monkeypatch) -> None:
    monkeypatch.setenv("EDUGRADE_ENV", "production")
    monkeypatch.delenv("EDUGRADE_MATH_VERIFY_TLS_CERT_FILE", raising=False)
    monkeypatch.delenv("EDUGRADE_MATH_VERIFY_TLS_KEY_FILE", raising=False)

    with pytest.raises(ValueError, match="TLS certificate"):
        main()
