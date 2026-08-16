from math_verification_worker import server


def test_timeout_returns_unknown():
    result = server.run_bounded("__test_sleep__", {"seconds": 2}, 0.05)
    assert result["status"] == "uncertain"
    assert result["reason_code"] == "verification_timeout"
