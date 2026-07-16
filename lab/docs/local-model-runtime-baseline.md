# Local Model Runtime Baseline

Status: Story H accepted after actual model integrity and measured benchmark review.

## Spec v2 Decision

Use the official Qwen `Qwen3-4B-Q4_K_M.gguf` with a pinned portable Windows CPU build of `llama.cpp`. The runtime, model, and download cache stay inside ignored lab directories. The initial benchmark is CPU-only, six threads, one concurrent request, 4096-token context, and deterministic generation settings.

The original `Qwen3-4B-Instruct-2507` preference is not used for this baseline because its official GGUF endpoint was unavailable from this environment. The official Qwen3 4B GGUF is reachable from ModelScope and supports non-thinking behavior through prompt control.

## Safety And Reproducibility

- Runtime release, official source, transport mirror, download size, and SHA256 are pinned. The mirror is used because the official GitHub asset connection resets in this environment.
- Model size and SHA256 are pinned.
- Model artifacts are never committed to Git.
- Preparation fails on a size or hash mismatch.
- The baseline does not claim GPU acceleration.
- Benchmark results must record machine, runtime, model, arguments, and raw runs.

## Commands

```powershell
npm.cmd run runtime:prepare
npm.cmd run runtime:check
npm.cmd run benchmark:local
```

## Acceptance

- `llama-server.exe` and `llama-bench.exe` exist under `lab/.runtime`.
- The 4B model exists under `lab/.models` and matches SHA256.
- The benchmark completes three prompt and generation measurements.
- The measured report is reviewed before Story I starts.

## Measured Result

Measured on 2026-07-15 with Ryzen 5 5500U, six CPU threads, no GPU layers, and `llama.cpp` build 10012:

| Test | Average throughput | Measured average time |
| --- | ---: | ---: |
| 512 prompt tokens | 25.41 tokens/s | 20.28 s |
| 128 generated tokens | 6.86 tokens/s | 19.06 s |

The GGUF file is 2,497,280,256 bytes and its SHA256 is `7485fe6f11af29433bc51cab58009521f205840f5b4ae3a32fa7f92e8534fdf5`.

## Review Decision

The model is viable for offline evaluation and one-at-a-time teacher assistance. It is not a production concurrency baseline: a representative 512-prompt/128-generation request takes about 39 seconds before application overhead. Story I must keep outputs compact, enforce timeouts, and report latency rather than hiding it. A server or faster machine is required before wider production throughput claims.
