from math_verification_worker.parser import parse_restricted_latex
from math_verification_worker.verifier import verify_equivalence, verify_transition


def test_algebraic_equivalence_and_solution_completeness():
    expanded = parse_restricted_latex("x^2+x")
    factored = parse_restricted_latex("x(x+1)")
    assert verify_equivalence(expanded, factored).status == "verified"
    complete = parse_restricted_latex("x^2=4")
    incomplete = parse_restricted_latex("x=2")
    result = verify_transition(complete, incomplete)
    assert result.status == "contradicted"
    assert result.relation == "solutions_lost"


def test_domain_assumptions_prevent_unsafe_radical_simplification():
    radical = parse_restricted_latex(r"\sqrt{x^2}")
    plain = parse_restricted_latex("x")
    assert verify_equivalence(radical, plain, {"variables": {"x": "real"}}).status != "verified"
    assert verify_equivalence(radical, plain, {"variables": {"x": "positive_real"}}).status == "verified"
