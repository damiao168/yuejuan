import pytest
from grading_agent.errors import AgentError
from grading_agent.paper_parser import PaperParser


class FakeStructuredModel:
    class _Session:
        def __enter__(self):
            return None

        def __exit__(self, *_args):
            return False

    def session(self, _request_id):
        return self._Session()

    def request_structured(self, _request_id, _messages, _schema, _name):
        return {
            "questions": [{
                "question_no": "1", "question_type": "calculation", "score": 10,
                "stem": "计算", "knowledge_points": ["函数"],
                "answer_key": {"standard_answer": "2", "equivalent_answers": [], "tolerance": {}},
                "rubric": {"status": "draft", "max_score": 10, "points": [{"id": "p1", "description": "过程正确", "score": 10, "required": True}], "deductions": [], "examples": []},
                "confidence": 0.9, "issues": [],
            }],
            "issues": [],
        }


def test_parses_document_pair_into_reviewable_questions():
    output = PaperParser(FakeStructuredModel()).parse({"request_id": "job-1", "subject": "数学", "paper_text": "第一题，请计算函数结果。" * 3, "answer_text": "第一题答案为2。"})
    assert output["questions"][0]["question_type"] == "calculation"


def test_rejects_incomplete_documents():
    with pytest.raises(AgentError):
        PaperParser(FakeStructuredModel()).parse({"request_id": "job-1", "subject": "语文", "paper_text": "短", "answer_text": "答案"})
