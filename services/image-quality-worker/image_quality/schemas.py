from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class QualityAnalysisResult:
    quality_status: str
    quality_report: dict[str, Any]
    quality_issues: list[dict[str, Any]]
    normalization_transform: dict[str, Any]
    normalized_png: bytes = field(repr=False)

