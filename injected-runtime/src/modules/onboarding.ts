import type { RuntimeModule, RuntimeModuleContext } from "../core/moduleRegistry";

export const ONBOARDING_MODULE_ID = "onboarding";
export const ONBOARDING_TIMER_ID = "onboarding.accept";
export const ONBOARDING_INTERVAL_MS = 500;
export const ONBOARDING_MAX_ATTEMPTS = 240;

function buttonLabel(button: HTMLButtonElement): string {
  return (button.innerText || button.textContent || "").trim();
}

export function findAcceptButton(doc: Document): HTMLButtonElement | null {
  for (const button of Array.from(doc.querySelectorAll<HTMLButtonElement>("button"))) {
    if (!/^accept and continue$/i.test(buttonLabel(button))) continue;
    if (button.disabled) continue;
    return button;
  }
  return null;
}

class OnboardingModule implements RuntimeModule {
  readonly id = ONBOARDING_MODULE_ID;
  private doc: Document | null = null;
  private accepted = false;
  private attempts = 0;
  private schedulerUnsubscribe: (() => void) | null = null;
  private timerUnsubscribe: (() => void) | null = null;

  init(context: RuntimeModuleContext): void {
    this.doc = context.document;
    this.schedulerUnsubscribe = context.scheduler.on("button", () => {
      this.accept();
    });
    this.timerUnsubscribe = context.timers.every(
      ONBOARDING_TIMER_ID,
      ONBOARDING_INTERVAL_MS,
      () => {
        this.attempts += 1;
        if (this.accept() || this.attempts > ONBOARDING_MAX_ATTEMPTS) this.stopSignals();
      },
      { hidden: "pause" },
    );
  }

  start(): void {
    this.accept();
  }

  dispose(): void {
    this.stopSignals();
  }

  private accept(): boolean {
    if (this.accepted) return true;
    if (!this.doc) return false;
    const button = findAcceptButton(this.doc);
    if (!button) return false;
    button.click();
    this.accepted = true;
    this.stopSignals();
    return true;
  }

  private stopSignals(): void {
    this.schedulerUnsubscribe?.();
    this.schedulerUnsubscribe = null;
    this.timerUnsubscribe?.();
    this.timerUnsubscribe = null;
  }
}

export function createOnboardingModule(): RuntimeModule {
  return new OnboardingModule();
}
