from __future__ import annotations

import io

from page_processing.config import Config
from page_processing.runner import Runner
from PIL import Image


class FakeClient:
    def __init__(self, image: bytes) -> None:
        self.image = image
        self.completed = None
        self.uploads = []

    def download(self, path: str) -> bytes:
        return self.image

    def upload_asset(self, task: dict, owner_type: str, owner_id: str, filename: str, data: bytes) -> dict:
        self.uploads.append((owner_type, owner_id, filename))
        return {"id": f"asset-{len(self.uploads)}", "hash_sha256": f"hash-{len(self.uploads)}"}

    def complete_paper_import_decode(self, import_id: str, payload: dict) -> None:
        self.completed = (import_id, payload)


def test_layout_task_renders_paper_import_for_existing_ocr_worker() -> None:
    image = Image.new("RGB", (320, 200), "white")
    output = io.BytesIO(); image.save(output, "PNG")
    client = FakeClient(output.getvalue())
    runner = Runner(client, Config("http://api", "demo", "worker", "secret", "worker-1"))
    runner._process_paper_import_decode({
        "id": "task-1", "lease_token": "lease-1", "source_type": "paper_import_job",
        "payload": {"paper_import_id": "import-1", "exam_id": "exam-1", "documents": [
            {"role": "paper", "download_url": "/files/paper", "content_type": "image/png", "max_pages": 10},
        ]},
    })
    assert client.completed is not None
    import_id, result = client.completed
    assert import_id == "import-1"
    assert result["pages"][0]["role"] == "paper"
    assert client.uploads[0][0:2] == ("import", "import-1")
