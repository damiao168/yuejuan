import pytest
from grading_agent.errors import AgentError
from grading_agent.paper_parser import PaperParser


def ref(source_id="source-1"):
    return {"source_id": source_id, "file_asset_id": "file-1", "document_index": 0, "page_no": None, "block_id": None, "bbox": None, "text_start": 0, "text_end": 1, "ocr_confidence": None}


class FakeStructuredModel:
    class _Session:
        def __enter__(self): return None
        def __exit__(self, *_args): return False

    def __init__(self, output): self.output = output
    def session(self, _request_id): return self._Session()
    def request_structured(self, *_args): return self.output


def output(role, questions=None, answers=None, solutions=None):
    return {"documents": [{"source_id": "source-1", "detected_role": role, "role_confidence": .98}], "question_candidates": questions or [], "answer_candidates": answers or [], "solution_candidates": solutions or [], "issues": []}


def payload(content="1.A"):
    return {"request_id": "job-1", "subject": "数学", "documents": [{"source_id": "source-1", "file_asset_id": "file-1", "document_index": 0, "role_hint": "auto", "content": content, "blocks": []}]}


def test_answer_only_is_valid_and_does_not_hallucinate_question():
    answer = {"candidate_id": "a1", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "standard_answer": "A", "equivalent_answers": [], "tolerance": None, "confidence": .95, "source_refs": [ref()], "issues": []}
    result = PaperParser(FakeStructuredModel(output("answer", answers=[answer]))).parse(payload())
    assert result["question_candidates"] == []
    assert result["answer_candidates"][0]["standard_answer"] == "A"


def test_question_only_allows_unknown_score_answer_and_rubric():
    question = {"candidate_id": "q1", "question_no_raw": "第1题", "question_no_normalized": "1", "parent_question_no": None, "subquestion_no": None, "section_hint": None, "stem": "计算", "options": [], "question_type": "calculation", "score": None, "knowledge_point_hints": [], "confidence": .9, "source_refs": [ref()], "issues": []}
    result = PaperParser(FakeStructuredModel(output("question", questions=[question]))).parse(payload("第1题 计算"))
    assert result["question_candidates"][0]["score"] is None
    assert result["answer_candidates"] == []
    assert "rubric" not in result["question_candidates"][0]


def test_solution_only_is_valid():
    solution = {"candidate_id": "s1", "question_no_hint": "18(1)", "question_no_normalized": "18(1)", "subquestion_no_hint": "1", "raw_text": "因为…所以…", "steps": [{"step_no": 1, "content": "因为…"}], "confidence": .88, "source_refs": [ref()], "issues": []}
    result = PaperParser(FakeStructuredModel(output("solution", solutions=[solution]))).parse(payload("第18题解析"))
    assert result["solution_candidates"][0]["question_no_normalized"] == "18(1)"


def test_rejects_empty_document_collection():
    with pytest.raises(AgentError): PaperParser(FakeStructuredModel(output("unknown"))).parse({"request_id": "job", "subject": "语文", "documents": []})


def test_rejects_duplicate_source_identity_or_document_order():
    request = payload()
    request["documents"].append({**request["documents"][0], "source_id": "source-2"})
    with pytest.raises(AgentError) as raised:
        PaperParser(FakeStructuredModel(output("unknown"))).parse(request)
    assert raised.value.code == "invalid_request"


def test_rejects_untrusted_provenance_from_model():
    answer = {"candidate_id": "a1", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "standard_answer": "A", "equivalent_answers": [], "tolerance": None, "confidence": .95, "source_refs": [ref("invented")], "issues": []}
    with pytest.raises(AgentError): PaperParser(FakeStructuredModel(output("answer", answers=[answer]))).parse(payload())


def test_rejects_source_ref_with_fabricated_file_or_document_index():
    fabricated = {**ref(), "file_asset_id": "invented", "document_index": 9}
    answer = {"candidate_id": "a1", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "standard_answer": "A", "equivalent_answers": [], "tolerance": None, "confidence": .95, "source_refs": [fabricated], "issues": []}
    with pytest.raises(AgentError): PaperParser(FakeStructuredModel(output("answer", answers=[answer]))).parse(payload())


def test_rejects_duplicate_candidate_ids_across_candidate_kinds():
    answer = {"candidate_id": "same", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "standard_answer": "A", "equivalent_answers": [], "tolerance": None, "confidence": .95, "source_refs": [ref()], "issues": []}
    solution = {"candidate_id": "same", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "raw_text": "解析", "steps": [], "confidence": .8, "source_refs": [ref()], "issues": []}
    with pytest.raises(AgentError): PaperParser(FakeStructuredModel(output("mixed", answers=[answer], solutions=[solution]))).parse(payload())


def test_rejects_ocr_ref_with_fabricated_confidence():
    ocr_ref = {**ref(), "page_no": 1, "block_id": "b1", "bbox": [1, 2, 3, 4], "text_start": None, "text_end": None, "ocr_confidence": .99}
    answer = {"candidate_id": "a1", "question_no_hint": "1", "question_no_normalized": "1", "subquestion_no_hint": None, "standard_answer": "A", "equivalent_answers": [], "tolerance": None, "confidence": .95, "source_refs": [ocr_ref], "issues": []}
    request = payload()
    request["documents"][0]["blocks"] = [{"source_id": "source-1", "document_index": 0, "block_id": "b1", "page_no": 1, "text": "1.A", "bbox": [1, 2, 3, 4], "confidence": .82}]
    with pytest.raises(AgentError): PaperParser(FakeStructuredModel(output("answer", answers=[answer]))).parse(request)
