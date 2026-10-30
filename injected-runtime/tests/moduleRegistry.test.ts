import test from "node:test";
import assert from "node:assert/strict";
import type { Broker } from "../src/core/broker";
import type { DomScheduler } from "../src/core/domScheduler";
import { ModuleRegistry, type RuntimeModule, type RuntimeModuleContext } from "../src/core/moduleRegistry";
import type { TimerRegistry } from "../src/core/timerRegistry";

test("module registry isolates lifecycle and dispatch failures", () => {
  const events: string[] = [];
  const logs: string[] = [];
  const good: RuntimeModule = {
    id: "good",
    init: () => events.push("good:init"),
    start: () => events.push("good:start"),
    dispatch: () => events.push("good:dispatch"),
    dispose: () => events.push("good:dispose"),
  };
  const badInit: RuntimeModule = {
    id: "bad-init",
    init: () => { throw new Error("init boom"); },
  };
  const badStart: RuntimeModule = {
    id: "bad-start",
    start: () => { throw new Error("start boom"); },
  };
  const badDispatch: RuntimeModule = {
    id: "bad-dispatch",
    dispatch: () => { throw new Error("dispatch boom"); },
  };
  const badDispose: RuntimeModule = {
    id: "bad-dispose",
    dispose: () => { throw new Error("dispose boom"); },
  };
  const context = {
    document: {} as Document,
    scheduler: {} as DomScheduler,
    timers: {} as TimerRegistry,
    broker: {} as Broker,
  };
  const registry = new ModuleRegistry(
    [good, badInit, badStart, badDispatch, badDispose],
    context,
    (message) => logs.push(message),
  );

  registry.init();
  registry.start();
  registry.dispatch({ type: "fixture" });
  registry.dispose();

  assert.deepEqual(events, ["good:init", "good:start", "good:dispatch", "good:dispose"]);
  assert.ok(logs.includes("[SwissKnife:bad-init] init failed"));
  assert.ok(logs.includes("[SwissKnife:bad-start] start failed"));
  assert.ok(logs.includes("[SwissKnife:bad-dispatch] dispatch failed"));
  assert.ok(logs.includes("[SwissKnife:bad-dispose] dispose failed"));
});
