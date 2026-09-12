import pytest

from grading_agent.errors import AgentError
from grading_agent.paper_compact import (
    MAX_BLOCKS_PER_CHUNK,
    anchored_paper_result,
    build_compact_chunks,
    expand_compact_output,
)


def _document(blocks):
    return {
        "source_id": "source-1",
        "file_asset_id": "file-1",
        "document_index": 0,
        "role_hint": "auto",
        "content": "\n".join(block["text"] for block in blocks),
        "blocks": blocks,
    }


def _block(block_id, text, bbox):
    return {
        "source_id": "source-1",
        "document_index": 0,
        "page_no": 1,
        "block_id": block_id,
        "text": text,
        "bbox": bbox,
        "confidence": 0.91,
    }


def test_two_column_page_is_converted_to_column_major_reading_order():
    blocks = [_block("header", "试卷标题", [100, 10, 800, 30])]
    for row in range(4):
        # Detector order is row-major and therefore interleaves both columns.
        blocks.extend(
            [
                _block(f"right-{row}", f"右栏{row}", [600, 100 + row * 40, 300, 20]),
                _block(f"left-{row}", f"左栏{row}", [50, 100 + row * 40, 300, 20]),
            ]
        )

    chunks = build_compact_chunks([_document(blocks)])
    ordered_text = [
        text for chunk in chunks for _, text in chunk.model_document["ordered_blocks"]
    ]

    assert ordered_text == [
        "试卷标题",
        "左栏0",
        "左栏1",
        "左栏2",
        "左栏3",
        "右栏0",
        "右栏1",
        "右栏2",
        "右栏3",
    ]


def test_three_column_page_is_recursively_ordered_by_geometry():
    blocks = []
    for row in range(4):
        # Real detectors often emit same-row boxes before finishing a column.
        blocks.extend(
            [
                _block(f"middle-{row}", f"中栏{row}", [360, 80 + row * 35, 220, 18]),
                _block(f"right-{row}", f"右栏{row}", [670, 80 + row * 35, 220, 18]),
                _block(f"left-{row}", f"左栏{row}", [50, 80 + row * 35, 220, 18]),
            ]
        )

    chunks = build_compact_chunks([_document(blocks)])
    ordered_text = [
        text for chunk in chunks for _, text in chunk.model_document["ordered_blocks"]
    ]

    assert ordered_text == [
        *(f"左栏{row}" for row in range(4)),
        *(f"中栏{row}" for row in range(4)),
        *(f"右栏{row}" for row in range(4)),
    ]


def test_dense_page_is_split_into_bounded_chunks():
    blocks = [
        _block(f"b{index}", f"{index}. " + "题目内容" * 8, [50, index * 25, 500, 20])
        for index in range(1, 80)
    ]

    chunks = build_compact_chunks([_document(blocks)])

    assert len(chunks) > 1
    assert (
        max(len(chunk.model_document["ordered_blocks"]) for chunk in chunks)
        <= MAX_BLOCKS_PER_CHUNK
    )
    assert sum(len(chunk.references) for chunk in chunks) == len(blocks)


def test_compact_references_are_hydrated_from_trusted_ocr_values():
    block = _block("trusted-block", "1. 计算", [10, 20, 30, 40])
    chunk = build_compact_chunks([_document([block])])[0]
    output = {
        "documents": [
            {
                "source_id": "source-1",
                "detected_role": "question",
                "role_confidence": 0.95,
            }
        ],
        "question_candidates": [
            {
                "question_no_raw": "1",
                "question_no_normalized": "1",
                "parent_question_no": None,
                "subquestion_no": None,
                "section_hint": None,
                "stem": "计算",
                "options": [],
                "question_type": "calculation",
                "score": None,
                "knowledge_point_hints": [],
                "confidence": 0.9,
                "source_refs": ["r001"],
                "issues": [],
            }
        ],
        "answer_candidates": [],
        "solution_candidates": [],
        "rubric_candidates": [],
        "issues": [],
    }

    expanded = expand_compact_output(output, chunk, 2)
    question = expanded["question_candidates"][0]

    assert question["candidate_id"] == "q-002-001"
    assert question["source_refs"][0]["block_id"] == "trusted-block"
    assert question["source_refs"][0]["bbox"] == [10, 20, 30, 40]
    assert question["source_refs"][0]["ocr_confidence"] == 0.91


def test_model_cannot_invent_a_compact_reference():
    chunk = build_compact_chunks(
        [_document([_block("trusted-block", "1. 计算", [10, 20, 30, 40])])]
    )[0]
    output = {
        "documents": [],
        "question_candidates": [
            {
                "source_refs": ["r999"],
            }
        ],
        "answer_candidates": [],
        "solution_candidates": [],
        "rubric_candidates": [],
        "issues": [],
    }

    with pytest.raises(AgentError) as raised:
        expand_compact_output(output, chunk, 1)

    assert raised.value.code == "model_output_invalid"


def test_explicit_number_answer_and_solution_markers_use_rule_fast_path():
    blocks = [
        _block("q1", "1. 已知集合A，求A的补集（ ）", [50, 100, 500, 20]),
        _block("o1", "A. R B. 空集 C. A D. 无法确定", [50, 130, 500, 20]),
        _block("a1", "【答案】B", [50, 160, 100, 20]),
        _block("s1", "【详解】根据补集定义可得。", [50, 190, 400, 20]),
        _block("q2", "2. 计算1+1（ ）", [50, 230, 500, 20]),
        _block("o2", "A. 0 B. 1 C. 2 D. 3", [50, 260, 500, 20]),
        _block("a2", "【答案】C", [50, 290, 100, 20]),
    ]
    result = anchored_paper_result([_document(blocks)])

    assert result is not None
    assert [
        item["question_no_normalized"] for item in result["question_candidates"]
    ] == [
        "1",
        "2",
    ]
    assert [item["standard_answer"] for item in result["answer_candidates"]] == [
        "B",
        "C",
    ]
    assert result["solution_candidates"][0]["raw_text"] == "根据补集定义可得。"
    assert result["answer_candidates"][0]["source_refs"][0]["block_id"] == "a1"


def test_rule_fast_path_declines_unanchored_or_sparse_answer_material():
    blocks = [
        _block("q1", "1. 简述函数概念", [50, 100, 500, 20]),
        _block("q2", "2. 简述导数概念", [50, 140, 500, 20]),
        _block("q3", "3. 简述数列概念", [50, 180, 500, 20]),
        _block("a3", "【答案】略", [50, 220, 100, 20]),
    ]

    assert anchored_paper_result([_document(blocks)]) is None
