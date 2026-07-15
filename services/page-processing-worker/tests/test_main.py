from __future__ import annotations

import json

import pytest

from page_processing.config import Config
import page_processing.__main__ as worker_main


class FakeClient:
    def __init__(self, *args: object) -> None:
        self.args = args
        self.logged_in = False

    def login(self) -> None:
        self.logged_in = True


class FakeRunner:
    def __init__(self, client: FakeClient, config: Config) -> None:
        self.client = client
        self.config = config
        self.forever_called = False
        self.once_called = False

    def run_once(self) -> int:
        self.once_called = True
        return 2

    def run_forever(self) -> None:
        self.forever_called = True


def _config(*, username: str = "worker", password: str = "secret") -> Config:
    return Config("http://api", "demo", username, password, "worker-1")


def test_once_claims_one_batch_and_prints_a_machine_readable_result(monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]) -> None:
    runner_instances: list[FakeRunner] = []

    def new_runner(client: FakeClient, config: Config) -> FakeRunner:
        runner = FakeRunner(client, config)
        runner_instances.append(runner)
        return runner

    monkeypatch.setattr(worker_main, "load_config", lambda: _config())
    monkeypatch.setattr(worker_main, "Client", FakeClient)
    monkeypatch.setattr(worker_main, "Runner", new_runner)

    assert worker_main.main(["--once"]) == 0
    assert len(runner_instances) == 1
    assert runner_instances[0].client.logged_in is True
    assert runner_instances[0].once_called is True
    assert runner_instances[0].forever_called is False
    assert json.loads(capsys.readouterr().out) == {"claimed_tasks": 2, "mode": "once"}


def test_default_mode_preserves_the_long_running_worker(monkeypatch: pytest.MonkeyPatch) -> None:
    runner_instances: list[FakeRunner] = []

    def new_runner(client: FakeClient, config: Config) -> FakeRunner:
        runner = FakeRunner(client, config)
        runner_instances.append(runner)
        return runner

    monkeypatch.setattr(worker_main, "load_config", lambda: _config())
    monkeypatch.setattr(worker_main, "Client", FakeClient)
    monkeypatch.setattr(worker_main, "Runner", new_runner)

    assert worker_main.main([]) == 0
    assert len(runner_instances) == 1
    assert runner_instances[0].client.logged_in is True
    assert runner_instances[0].once_called is False
    assert runner_instances[0].forever_called is True


def test_credentials_are_required_before_client_or_runner_start(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(worker_main, "load_config", lambda: _config(username="", password=""))

    def fail_if_started(*args: object) -> object:
        raise AssertionError("client or runner must not start without credentials")

    monkeypatch.setattr(worker_main, "Client", fail_if_started)
    monkeypatch.setattr(worker_main, "Runner", fail_if_started)

    with pytest.raises(SystemExit, match="credentials are required"):
        worker_main.main(["--once"])
