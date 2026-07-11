import io
import json
from urllib import error
from unittest.mock import patch

from page_processing.api import Client


def test_upload_page_reuses_authorized_duplicate_asset() -> None:
    existing = {"id": "asset-1", "hash_sha256": "abc123"}
    response = json.dumps({"error": {"code": "duplicate_file"}, "existing_file": existing}).encode()
    conflict = error.HTTPError("http://api/files", 409, "Conflict", {}, io.BytesIO(response))
    client = Client("http://api", "demo", "worker", "secret", token="token")
    task = {"payload": {"capture_file_id": "capture-1", "exam_id": "exam-1"}}

    with patch("page_processing.api.request.urlopen", side_effect=conflict):
        assert client.upload_page(task, 2, b"png") == existing
