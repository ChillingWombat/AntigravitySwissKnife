export type HiddenTimerBehavior = "pause" | "slow" | "run";

export interface TimerOptions {
  hidden?: HiddenTimerBehavior;
  slowFactor?: number;
}

export interface TimerRegistryOptions {
  document?: Document;
  now?: () => number;
  autoStart?: boolean;
  tickMs?: number;
  logger?: (message: string, ...args: unknown[]) => void;
}

interface TimerTask {
  id: string;
  intervalMs: number;
  callback: () => void;
  hidden: HiddenTimerBehavior;
  slowFactor: number;
  nextAt: number;
}

export class TimerRegistry {
  private readonly doc: Document;
  private readonly now: () => number;
  private readonly tickMs: number;
  private readonly logger: (message: string, ...args: unknown[]) => void;
  private readonly tasks = new Map<string, TimerTask>();
  private loopHandle: ReturnType<typeof setInterval> | null = null;
  private visible: boolean;

  constructor(options: TimerRegistryOptions = {}) {
    this.doc = options.document ?? document;
    this.now = options.now ?? (() => Date.now());
    this.tickMs = options.tickMs ?? 100;
    this.logger = options.logger ?? ((message, ...args) => console.error(message, ...args));
    this.visible = this.doc.visibilityState !== "hidden";
    this.doc.addEventListener("visibilitychange", this.handleVisibilityChange);
    if (options.autoStart ?? false) this.start();
  }

  every(id: string, intervalMs: number, callback: () => void, options: TimerOptions = {}): () => void {
    if (intervalMs <= 0) throw new Error("timer interval must be positive");
    const slowFactor = options.slowFactor ?? 4;
    if (slowFactor < 1) throw new Error("timer slow factor must be at least one");
    this.unregister(id);
    this.tasks.set(id, {
      id,
      intervalMs,
      callback,
      hidden: options.hidden ?? "pause",
      slowFactor,
      nextAt: this.now() + intervalMs,
    });
    return () => this.unregister(id);
  }

  unregister(id: string): void {
    this.tasks.delete(id);
  }

  setVisibility(visible: boolean): void {
    if (this.visible === visible) return;
    this.visible = visible;
    if (!visible) return;
    const now = this.now();
    for (const task of this.tasks.values()) {
      if (task.hidden === "pause") task.nextAt = now + task.intervalMs;
    }
  }

  start(): void {
    if (this.loopHandle !== null) return;
    this.loopHandle = setInterval(() => this.tick(), this.tickMs);
  }

  tick(now = this.now()): void {
    for (const task of Array.from(this.tasks.values())) {
      if (!this.visible && task.hidden === "pause") continue;
      if (now < task.nextAt) continue;
      try {
        task.callback();
      } catch (error) {
        this.logger(`[SwissKnife:timerRegistry] ${task.id} failed`, error);
      }
      const delay = this.visible || task.hidden === "run"
        ? task.intervalMs
        : task.intervalMs * task.slowFactor;
      task.nextAt = now + delay;
    }
  }

  stop(): void {
    this.doc.removeEventListener("visibilitychange", this.handleVisibilityChange);
    if (this.loopHandle !== null) {
      clearInterval(this.loopHandle);
      this.loopHandle = null;
    }
    this.tasks.clear();
  }

  private readonly handleVisibilityChange = (): void => {
    this.setVisibility(this.doc.visibilityState !== "hidden");
  };
}
