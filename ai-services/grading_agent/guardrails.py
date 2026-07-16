import re
import unicodedata


_CHINESE_PUNCTUATION = re.compile(r"[，。！？；：‘’“”（）【】《》、]")
_ASCII_PUNCTUATION = re.compile(r"[.,!?;:'\"()\[\]{}<>/\\|\-]")
_ZERO_WIDTH = re.compile(r"[\u200B-\u200D\uFEFF]")
_SPACE = re.compile(r"\s+")

_INJECTION_RULES = (
    ("ignore_rubric_cn", re.compile(r"忽略(?:之前|以上|所有)?(?:的)?(?:评分|规则|标准|指令)", re.I)),
    ("full_score_cn", re.compile(r"(?:给我|直接给|应该给)(?:满分|\d+(?:\.\d+)?分)", re.I)),
    ("role_override_cn", re.compile(r"你现在是(?:老师|教师|系统|管理员|开发者)", re.I)),
    ("schema_bypass_cn", re.compile(r"(?:输出.*schema.*之外|不要告诉老师)", re.I)),
    ("ignore_rubric_en", re.compile(r"ignore(?:all|previous|the)?(?:rules|instructions|rubric|gradingcriteria)", re.I)),
    ("full_score_en", re.compile(r"givemefull(?:marks|score|credit)", re.I)),
    ("role_override_en", re.compile(r"youarenow(?:the)?(?:teacher|system|admin|developer)", re.I)),
)


def normalize_evidence_text(value):
    text = unicodedata.normalize("NFKC", str(value or "")).casefold()
    text = _CHINESE_PUNCTUATION.sub("", text)
    text = _ASCII_PUNCTUATION.sub("", text)
    return _SPACE.sub("", text)


def detect_prompt_injection(value):
    text = unicodedata.normalize("NFKC", str(value or ""))
    text = _ZERO_WIDTH.sub("", text).casefold()
    text = _SPACE.sub("", text)
    matches = [name for name, pattern in _INJECTION_RULES if pattern.search(text)]
    return {"detected": bool(matches), "signals": matches}
