from math_verification_worker.parser import parse_restricted_latex
from math_verification_worker.verifier import (
    solve,
    verify_equivalence,
    verify_transition,
)


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


def test_cancellation_preserves_excluded_denominator_values():
    quotient = parse_restricted_latex(r"\frac{x}{x}")
    one = parse_restricted_latex("1")
    result = verify_equivalence(quotient, one)
    assert result.status == "contradicted"
    assert result.relation == "defined_domain_changed"
    equation = parse_restricted_latex(r"\frac{x^2}{x}=0")
    assert solve(equation, "x")["solution_set"] == "EmptySet"


def test_radical_domain_and_unsupported_constraints_fail_closed():
    equation = parse_restricted_latex(r"\sqrt{x-2}=0")
    assert solve(equation, "x")["solution_set"] == "{2}"
    result = verify_equivalence(equation, equation, {"constraints": ["x>3"]})
    assert result.status == "uncertain"
    assert solve(equation, "x", {"constraints": ["x>3"]})["status"] == "uncertain"
