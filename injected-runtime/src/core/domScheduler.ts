export type SelectorHandler = (element: Element) => void;

export interface DomSchedulerOptions {
  document?: Document;
  requestFrame?: (callback: FrameRequestCallback) => number;
  cancelFrame?: (handle: number) => void;
  observerFactory?: (callback: MutationCallback) => MutationObserver;
}

export class DomScheduler {
  private readonly doc: Document;
  private readonly requestFrame: (callback: FrameRequestCallback) => number;
  private readonly cancelFrame: (handle: number) => void;
  private readonly observerFactory: (callback: MutationCallback) => MutationObserver;
  private readonly routes = new Map<string, Set<SelectorHandler>>();
  private readonly pending = new Set<Element>();
  private observer: MutationObserver | null = null;
  private frameHandle: number | null = null;
  private visible: boolean;

  constructor(options: DomSchedulerOptions = {}) {
    this.doc = options.document ?? document;
    this.requestFrame = options.requestFrame ?? ((callback) => {
      const view = this.doc.defaultView;
      if (!view) throw new Error("window is unavailable for animation frames");
      return view.requestAnimationFrame(callback);
    });
    this.cancelFrame = options.cancelFrame ?? ((handle) => {
      this.doc.defaultView?.cancelAnimationFrame(handle);
    });
    this.observerFactory = options.observerFactory ?? ((callback) => new MutationObserver(callback));
    this.visible = this.doc.visibilityState !== "hidden";
    this.doc.addEventListener("visibilitychange", this.handleVisibilityChange);
  }

  get observerCount(): number {
    return this.observer ? 1 : 0;
  }

  on(selector: string, handler: SelectorHandler): () => void {
    let handlers = this.routes.get(selector);
    if (!handlers) {
      handlers = new Set();
      this.routes.set(selector, handlers);
    }
    handlers.add(handler);
    return () => {
      handlers?.delete(handler);
      if (handlers?.size === 0) this.routes.delete(selector);
    };
  }

  start(): boolean {
    if (this.observer) return true;
    const body = this.doc.body;
    if (!body) return false;
    this.observer = this.observerFactory(this.handleMutations);
    this.observer.observe(body, {
      childList: true,
      subtree: true,
      attributes: true,
    });
    return true;
  }

  stop(): void {
    this.doc.removeEventListener("visibilitychange", this.handleVisibilityChange);
    this.observer?.disconnect();
    this.observer = null;
    if (this.frameHandle !== null) {
      this.cancelFrame(this.frameHandle);
      this.frameHandle = null;
    }
    this.pending.clear();
    this.routes.clear();
  }

  private readonly handleMutations = (records: MutationRecord[]): void => {
    for (const record of records) {
      this.queueNode(record.target);
      for (const node of Array.from(record.addedNodes)) this.queueNode(node);
    }
    this.schedule();
  };

  private readonly handleVisibilityChange = (): void => {
    const visible = this.doc.visibilityState !== "hidden";
    if (!visible) {
      this.visible = false;
      if (this.frameHandle !== null) {
        this.cancelFrame(this.frameHandle);
        this.frameHandle = null;
      }
      return;
    }
    if (!this.visible) {
      this.visible = true;
      this.schedule();
    }
  };

  private queueNode(node: Node): void {
    if (node.nodeType === 1) this.pending.add(node as Element);
    if (node.parentElement) this.pending.add(node.parentElement);
  }

  private schedule(): void {
    if (!this.visible || this.pending.size === 0 || this.frameHandle !== null) return;
    this.frameHandle = this.requestFrame(() => {
      this.frameHandle = null;
      if (this.visible) this.flush();
    });
  }

  private flush(): void {
    const roots = Array.from(this.pending);
    this.pending.clear();
    for (const [selector, handlers] of this.routes) {
      const matched = new Set<Element>();
      for (const root of roots) {
        if (root.matches(selector)) matched.add(root);
        for (const element of Array.from(root.querySelectorAll(selector))) matched.add(element);
      }
      for (const element of matched) {
        for (const handler of Array.from(handlers)) {
          try {
            handler(element);
          } catch (error) {
            console.error(`[SwissKnife:domScheduler] selector ${selector} failed`, error);
          }
        }
      }
    }
  }
}
