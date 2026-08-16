import pytest
from math_verification_worker.parser import (
    UnsupportedExpression,
    parse_restricted_latex,
)


def test_parses_fraction_power_and_equation():
    ast = parse_restricted_latex(r"\frac{x+1}{2}=x^2")
    assert ast["kind"] == "equation"
    assert ast["children"][0]["kind"] == "fraction"
    assert ast["children"][1]["kind"] == "power"


def test_implicit_multiplication_is_explicit_in_ast():
    ast = parse_restricted_latex("2x+1")
    assert ast["children"][0]["value"] == "*"


@pytest.mark.parametrize("source", [r"\int x dx", r"\begin{matrix}1\end{matrix}", "x;import os", ""])
def test_unknown_constructs_fail_closed(source):
    with pytest.raises(UnsupportedExpression):
        parse_restricted_latex(source)
