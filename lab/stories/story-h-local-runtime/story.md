# Story H Local Model Runtime And Hardware Baseline

## Spec v1

Run a 4-bit Qwen 4B model on the current computer and measure whether it is a viable local grading baseline.

## Spec Review

A normal Ollama installation writes outside `lab`, the integrated AMD GPU has insufficient dedicated memory for a reliable full offload, and an unpinned model download would not be reproducible. A configuration-only story would not prove that the machine can run the model.

## Spec v2

Use a pinned portable CPU build of `llama.cpp`, keep all downloaded artifacts under ignored lab directories, verify the Qwen GGUF SHA256, and execute a repeatable CPU benchmark with raw results.

## Implementation

Added the runtime manifest, safe path validator, preparation script, integrity checker, benchmark runner, tests, and runtime documentation.

## Implementation Review

The runtime and 2.50 GB model were downloaded inside ignored lab directories. Full SHA256 matched the pinned official Qwen artifact. Three CPU repetitions measured 25.41 prompt tokens/s and 6.86 generation tokens/s. Review concluded that the machine is suitable for offline and single-request use but not production concurrency. The first runtime inspection also misleadingly reported `size_matches=false` for binaries that have no configured expected size.

## Implementation Fixes

Recorded the raw benchmark under `evals/reports`, added the measured conclusion to the runtime document, and changed unconfigured executable-size checks to report `null` rather than a false mismatch.

## Acceptance

Accepted. Runtime and model integrity pass, the real benchmark is reproducible, and the limited local-use conclusion is explicit.
