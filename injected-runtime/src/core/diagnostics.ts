export const RUNTIME_VERSION = "1.0.0";
export const DECLARED_TIMER_SITES = 1;

export interface RuntimeDiagnostics {
  version: string;
  moduleCount: number;
  observerCount: number;
  declaredTimerSites: number;
}

export function createDiagnostics(moduleCount: number, observerCount: number): RuntimeDiagnostics {
  return {
    version: RUNTIME_VERSION,
    moduleCount,
    observerCount,
    declaredTimerSites: DECLARED_TIMER_SITES,
  };
}
