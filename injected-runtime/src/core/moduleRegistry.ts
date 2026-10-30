import type { Broker } from "./broker";
import type { DomScheduler } from "./domScheduler";
import type { TimerRegistry } from "./timerRegistry";

export interface RuntimeModuleContext {
  document: Document;
  scheduler: DomScheduler;
  timers: TimerRegistry;
  broker: Broker;
}

export interface RuntimeEvent {
  type: string;
  target?: Element;
  data?: unknown;
}

export interface RuntimeModule {
  readonly id: string;
  init?(context: RuntimeModuleContext): void;
  start?(context: RuntimeModuleContext): void;
  dispatch?(event: RuntimeEvent): void;
  dispose?(): void;
}

type ModulePhase = "init" | "start" | "dispatch" | "dispose";

export class ModuleRegistry {
  private readonly modules: readonly RuntimeModule[];
  private readonly context: RuntimeModuleContext;
  private readonly logger: (message: string, ...args: unknown[]) => void;
  private disposed = false;

  constructor(
    modules: readonly RuntimeModule[],
    context: RuntimeModuleContext,
    logger: (message: string, ...args: unknown[]) => void = (message, ...args) => console.error(message, ...args),
  ) {
    const ids = new Set<string>();
    for (const module of modules) {
      if (ids.has(module.id)) throw new Error(`duplicate runtime module: ${module.id}`);
      ids.add(module.id);
    }
    this.modules = [...modules];
    this.context = context;
    this.logger = logger;
  }

  init(): void {
    for (const module of this.modules) {
      if (module.init) this.isolate(module, "init", () => module.init?.(this.context));
    }
  }

  start(): void {
    for (const module of this.modules) {
      if (module.start) this.isolate(module, "start", () => module.start?.(this.context));
    }
  }

  dispatch(event: RuntimeEvent): void {
    for (const module of this.modules) {
      if (module.dispatch) this.isolate(module, "dispatch", () => module.dispatch?.(event));
    }
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    for (const module of [...this.modules].reverse()) {
      if (module.dispose) this.isolate(module, "dispose", () => module.dispose?.());
    }
  }

  private isolate(module: RuntimeModule, phase: ModulePhase, operation: () => void): void {
    try {
      operation();
    } catch (error) {
      this.logger(`[SwissKnife:${module.id}] ${phase} failed`, error);
    }
  }
}
