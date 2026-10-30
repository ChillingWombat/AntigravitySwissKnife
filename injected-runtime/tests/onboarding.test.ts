import test from "node:test";
import assert from "node:assert/strict";
import type { DomScheduler } from "../src/core/domScheduler";
import type { RuntimeModuleContext } from "../src/core/moduleRegistry";
import type { TimerRegistry } from "../src/core/timerRegistry";
import { createOnboardingModule, ONBOARDING_INTERVAL_MS, ONBOARDING_TIMER_ID } from "../src/modules/onboarding";
import { onboardingFixtureButtons } from "./fixtures/onboarding";

test("onboarding fixture clicks only Accept and continue once", () => {
  const clicks = new Map<string, number>();
  const buttons = onboardingFixtureButtons.map((fixture) => {
    clicks.set(fixture.label, 0);
    return {
      innerText: fixture.label,
      textContent: fixture.label,
      disabled: false,
      click() {
        clicks.set(fixture.label, (clicks.get(fixture.label) ?? 0) + 1);
      },
    } as unknown as HTMLButtonElement;
  });
  const documentValue = {
    querySelectorAll(selector: string) {
      assert.equal(selector, "button");
      return buttons;
    },
  } as unknown as Document;
  const selectorHandlers: Array<() => void> = [];
  const timerHandlers: Array<() => void> = [];
  let schedulerUnsubscribes = 0;
  let timerUnsubscribes = 0;
  const context = {
    document: documentValue,
    scheduler: {
      on(selector: string, handler: () => void) {
        assert.equal(selector, "button");
        selectorHandlers.push(handler);
        return () => { schedulerUnsubscribes += 1; };
      },
    } as unknown as DomScheduler,
    timers: {
      every(id: string, intervalMs: number, handler: () => void) {
        assert.equal(id, ONBOARDING_TIMER_ID);
        assert.equal(intervalMs, ONBOARDING_INTERVAL_MS);
        timerHandlers.push(handler);
        return () => { timerUnsubscribes += 1; };
      },
    } as unknown as TimerRegistry,
    broker: {} as RuntimeModuleContext["broker"],
  } as RuntimeModuleContext;
  const module = createOnboardingModule();

  module.init?.(context);
  module.start?.(context);
  module.start?.(context);
  selectorHandlers[0]?.();
  timerHandlers[0]?.();

  assert.equal(clicks.get("Accept and continue"), 1);
  assert.equal(clicks.get("Allow usage data"), 0);
  assert.equal(clicks.get("Cancel"), 0);
  assert.equal(schedulerUnsubscribes, 1);
  assert.equal(timerUnsubscribes, 1);
  module.dispose?.();
  assert.equal(schedulerUnsubscribes, 1);
  assert.equal(timerUnsubscribes, 1);
});
