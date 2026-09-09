import { describe, expect, it } from "vitest";
import { ExclusiveCommandGate, LatestRequestGate, runCaptureCommand } from "./captureWorkflow";

describe("capture workflow coordination", () => {
  it("ignores stale query responses after refresh or exam switch", () => {
    const gate = new LatestRequestGate();
    const first = gate.begin();
    const refresh = gate.begin();
    expect(gate.isCurrent(first)).toBe(false);
    expect(gate.isCurrent(refresh)).toBe(true);
    gate.invalidate();
    expect(gate.isCurrent(refresh)).toBe(false);
  });

  it("suppresses duplicate commands synchronously and reloads only after success", async () => {
    const gate = new ExclusiveCommandGate();
    const events: string[] = [];
    let finishCommand!: () => void;
    const commandFinished = new Promise<void>((resolve) => { finishCommand = resolve; });
    const first = runCaptureCommand({
      key: "ocr-submission-1", gate,
      command: async () => { events.push("command"); await commandFinished; events.push("committed"); },
      reload: async () => { events.push("reload"); }
    });
    const duplicate = await runCaptureCommand({
      key: "ocr-submission-1", gate,
      command: async () => { events.push("duplicate"); }
    });
    expect(duplicate).toBe("ignored");
    finishCommand();
    await expect(first).resolves.toBe("completed");
    expect(events).toEqual(["command", "committed", "reload"]);
  });

  it("does not reload a failed command and releases the gate for retry", async () => {
    const gate = new ExclusiveCommandGate();
    let reloads = 0;
    await expect(runCaptureCommand({
      key: "quality-1", gate,
      command: async () => { throw new Error("conflict"); },
      reload: async () => { reloads += 1; }
    })).rejects.toThrow("conflict");
    expect(reloads).toBe(0);
    await expect(runCaptureCommand({ key: "quality-1", gate, command: async () => undefined })).resolves.toBe("completed");
  });
});
