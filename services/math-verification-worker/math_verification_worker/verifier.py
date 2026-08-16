from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import sympy
from sympy import S
from sympy.core.relational import Relational


class VerificationError(ValueError):
    pass


@dataclass(frozen=True)
class VerificationResult:
    status: str
    relation: str
    domain: str
    constraints: tuple[str, ...]
    evidence: tuple[str, ...]
    engine: str = "sympy"
    engine_version: str = sympy.__version__
    ruleset_version: str = "yuejuan-math-rules-v1"


def normalize(ast: dict[str, Any], domain: dict[str, Any] | None = None) -> dict[str, Any]:
    expression = ast_to_sympy(ast, domain or {})
    return {"srepr": sympy.srepr(expression), "canonical": str(expression)}


def verify_equivalence(left_ast: dict[str, Any], right_ast: dict[str, Any], domain: dict[str, Any] | None = None) -> VerificationResult:
    assumptions = domain or {}
    left = ast_to_sympy(left_ast, assumptions); right = ast_to_sympy(right_ast, assumptions)
    domain_name = str(assumptions.get("domain", "real"))
    constraints = tuple(str(item) for item in assumptions.get("constraints", []))
    if left == right:
        return VerificationResult("verified", "exact_ast_equivalent", domain_name, constraints, ("exact_sympy_match",))
    if isinstance(left, sympy.Equality) and isinstance(right, sympy.Equality):
        variables = sorted(left.free_symbols | right.free_symbols, key=str)
        if len(variables) == 1:
            universe = S.Reals if domain_name == "real" else S.Complexes
            left_set = sympy.solveset(left, variables[0], domain=universe)
            right_set = sympy.solveset(right, variables[0], domain=universe)
            if left_set == right_set:
                return VerificationResult("verified", "solution_set_equivalent", domain_name, constraints, (str(left_set),))
            if left_set.is_subset(right_set) is True:
                return VerificationResult("contradicted", "extra_solutions_introduced", domain_name, constraints, (str(left_set), str(right_set)))
            if right_set.is_subset(left_set) is True:
                return VerificationResult("contradicted", "solutions_lost", domain_name, constraints, (str(left_set), str(right_set)))
            return VerificationResult("contradicted", "different_solution_set", domain_name, constraints, (str(left_set), str(right_set)))
    if not isinstance(left, Relational) and not isinstance(right, Relational):
        difference = sympy.cancel(sympy.together(left - right))
        if difference == 0:
            return VerificationResult("verified", "algebraically_equivalent", domain_name, constraints, ("together_cancel",))
        counterexample = _counterexample(left, right)
        if counterexample:
            return VerificationResult("contradicted", "numeric_counterexample", domain_name, constraints, (counterexample,))
    return VerificationResult("uncertain", "unknown", domain_name, constraints, ())


def verify_transition(previous_ast: dict[str, Any], next_ast: dict[str, Any], domain: dict[str, Any] | None = None) -> VerificationResult:
    result = verify_equivalence(previous_ast, next_ast, domain)
    if result.status == "verified":
        return VerificationResult("verified", "equivalent_transform", result.domain, result.constraints, result.evidence)
    return result


def solve(ast: dict[str, Any], variable: str, domain: dict[str, Any] | None = None) -> dict[str, Any]:
    assumptions = domain or {}; expression = ast_to_sympy(ast, assumptions)
    symbol = _symbols(assumptions).get(variable, sympy.Symbol(variable, real=True))
    universe = S.Reals if assumptions.get("domain", "real") == "real" else S.Complexes
    result = sympy.solveset(expression, symbol, domain=universe)
    return {"status": "verified", "solution_set": str(result), "engine": "sympy", "engine_version": sympy.__version__, "ruleset_version": "yuejuan-math-rules-v1"}


def ast_to_sympy(node: dict[str, Any], domain: dict[str, Any]) -> sympy.Basic:
    if not isinstance(node, dict): raise VerificationError("AST node must be an object")
    kind = node.get("kind"); value = str(node.get("value", "")); children = node.get("children", [])
    if not isinstance(children, list) or len(children) > 64: raise VerificationError("invalid AST children")
    parsed = [ast_to_sympy(child, domain) for child in children]
    if kind == "number": return sympy.Rational(value) if "." not in value else sympy.Float(value)
    if kind == "symbol":
        if not value.isalpha() or len(value) > 32: raise VerificationError("invalid symbol")
        return _symbols(domain).get(value, sympy.Symbol(value, real=True))
    if kind == "group" and len(parsed) == 1: return parsed[0]
    if kind == "operator" and len(parsed) == 2:
        if value == "+": return parsed[0] + parsed[1]
        if value == "-": return parsed[0] - parsed[1]
        if value == "*": return parsed[0] * parsed[1]
        raise VerificationError("unsupported operator")
    if kind == "fraction" and len(parsed) == 2: return parsed[0] / parsed[1]
    if kind == "power" and len(parsed) == 2: return parsed[0] ** parsed[1]
    if kind == "radical" and len(parsed) == 1: return sympy.sqrt(parsed[0])
    if kind == "function" and len(parsed) == 1 and value in {"sin", "cos", "tan", "abs"}:
        return {"sin": sympy.sin, "cos": sympy.cos, "tan": sympy.tan, "abs": sympy.Abs}[value](parsed[0])
    if kind == "equation" and len(parsed) == 2: return sympy.Eq(parsed[0], parsed[1])
    if kind == "inequality" and len(parsed) == 2:
        operations = {"<": sympy.Lt, ">": sympy.Gt, "\\le": sympy.Le, "\\leq": sympy.Le, "\\ge": sympy.Ge, "\\geq": sympy.Ge, "\\neq": sympy.Ne}
        if value in operations: return operations[value](parsed[0], parsed[1])
    raise VerificationError(f"unsupported AST kind: {kind}")


def _symbols(domain: dict[str, Any]) -> dict[str, sympy.Symbol]:
    out: dict[str, sympy.Symbol] = {}
    variables = domain.get("variables", {})
    if not isinstance(variables, dict): raise VerificationError("variables must be an object")
    for name, declared in variables.items():
        if not str(name).isalpha(): raise VerificationError("invalid variable name")
        kind = str(declared).lower()
        kwargs = {"real": True}
        if kind == "integer": kwargs = {"integer": True}
        elif kind == "positive_real": kwargs = {"real": True, "positive": True}
        elif kind not in {"real", "integer", "positive_real"}: raise VerificationError("unsupported variable domain")
        out[str(name)] = sympy.Symbol(str(name), **kwargs)
    return out


def _counterexample(left: sympy.Basic, right: sympy.Basic) -> str:
    variables = sorted(left.free_symbols | right.free_symbols, key=str)
    for value in (-3, -1, 0, 1, 2, 5):
        substitutions = {symbol: value + index for index, symbol in enumerate(variables)}
        try:
            if sympy.N(left.subs(substitutions) - right.subs(substitutions)) != 0:
                return ",".join(f"{symbol}={substitutions[symbol]}" for symbol in variables)
        except (TypeError, ValueError, ZeroDivisionError):
            continue
    return ""
