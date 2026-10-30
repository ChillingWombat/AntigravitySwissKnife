import { Broker } from "./core/broker";
import { createDiagnostics, RUNTIME_VERSION, type RuntimeDiagnostics } from "./core/diagnostics";
import { DomScheduler } from "./core/domScheduler";
import { ModuleRegistry, type RuntimeModuleContext } from "./core/moduleRegistry";
import { TimerRegistry } from "./core/timerRegistry";
import { createOnboardingModule } from "./modules/onboarding";
import { RUNTIME_STYLE_NAMESPACE } from "./styles";

export interface SwissRuntimeHandle {
  readonly version: string;
  readonly styleNamespace: string;
  readonly diagnostics: RuntimeDiagnostics;
  dispose(): void;
}

declare global {
  interface Window {
    __SWISS_RUNTIME__?: SwissRuntimeHandle;
  }
}

function disposeExistingRuntime(host: Window): void {
  const existing = host.__SWISS_RUNTIME__;
  if (!existing || typeof existing.dispose !== "function") return;
  try {
    existing.dispose();
  } catch (error) {
    console.error("[SwissKnife:runtime] previous runtime dispose failed", error);
  }
}

export function createRuntime(host: Window): SwissRuntimeHandle {
  disposeExistingRuntime(host);

  const scheduler = new DomScheduler({ document: host.document });
  const timers = new TimerRegistry({ document: host.document });
  const broker = new Broker(host);
  const context: RuntimeModuleContext = {
    document: host.document,
    scheduler,
    timers,
    broker,
  };
  const modules = new ModuleRegistry([createOnboardingModule()], context);

  modules.init();
  scheduler.start();
  timers.start();
  modules.start();

  const diagnostics = createDiagnostics(1, scheduler.observerCount);
  let disposed = false;
  let handle: SwissRuntimeHandle;
  handle = {
    version: RUNTIME_VERSION,
    styleNamespace: RUNTIME_STYLE_NAMESPACE,
    diagnostics,
    dispose(): void {
      if (disposed) return;
      disposed = true;
      modules.dispose();
      scheduler.stop();
      timers.stop();
      if (host.__SWISS_RUNTIME__ === handle) delete host.__SWISS_RUNTIME__;
    },
  };
  host.__SWISS_RUNTIME__ = handle;
  return handle;
}

if (typeof window !== "undefined" && window.document) {
  try {
    createRuntime(window);
  } catch (error) {
    console.error("[SwissKnife:runtime] bootstrap failed", error);
  }
}
