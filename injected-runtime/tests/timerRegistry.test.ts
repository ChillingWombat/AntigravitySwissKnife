import test from "node:test";
import assert from "node:assert/strict";
import { TimerRegistry } from "../src/core/timerRegistry";

function visibilityDocument() {
  let visibility: DocumentVisibilityState = "visible";
  const listeners: EventListener[] = [];
  const documentValue = {
    get visibilityState() {
      return visibility;
    },
    addEventListener(type: string, listener: EventListener) {
      if (type === "visibilitychange") listeners.push(listener);
    },
    removeEventListener(type: string, listener: EventListener) {
      if (type !== "visibilitychange") return;
      const index = listeners.indexOf(listener);
      if (index >= 0) listeners.splice(index, 1);
    },
  } as unknown as Document;
  return {
    document: documentValue,
    setVisible(visible: boolean) {
      visibility = visible ? "visible" : "hidden";
      for (const listener of listeners) listener(new Event("visibilitychange"));
    },
  };
}

test("timer registry pauses hidden timers without catch-up", () => {
  const fixture = visibilityDocument();
  let now = 0;
  let calls = 0;
  const timers = new TimerRegistry({
    document: fixture.document,
    now: () => now,
    autoStart: false,
  });
  timers.every("pause-test", 100, () => { calls += 1; }, { hidden: "pause" });

  fixture.setVisible(false);
  now = 100;
  timers.tick(now);
  assert.equal(calls, 0);

  fixture.setVisible(true);
  now = 199;
  timers.tick(now);
  assert.equal(calls, 0);
  now = 200;
  timers.tick(now);
  assert.equal(calls, 1);
  timers.stop();
});

test("timer registry slows hidden timers by the configured factor", () => {
  const fixture = visibilityDocument();
  let now = 0;
  let calls = 0;
  const timers = new TimerRegistry({
    document: fixture.document,
    now: () => now,
    autoStart: false,
  });
  timers.every("slow-test", 100, () => { calls += 1; }, { hidden: "slow", slowFactor: 4 });

  fixture.setVisible(false);
  now = 100;
  timers.tick(now);
  assert.equal(calls, 1);
  now = 499;
  timers.tick(now);
  assert.equal(calls, 1);
  now = 500;
  timers.tick(now);
  assert.equal(calls, 2);
  timers.stop();
});
