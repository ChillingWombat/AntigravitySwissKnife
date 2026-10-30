export type SwissRequestOptions = Record<string, unknown>;

export interface SwissBridge {
  request<T = unknown>(path: string, options?: SwissRequestOptions): Promise<T>;
}

type SwissWindow = Window & { swiss?: SwissBridge };

declare global {
  interface Window {
    swiss?: SwissBridge;
  }
}

export class Broker {
  private readonly host: SwissWindow;

  constructor(host: Window = window) {
    this.host = host as SwissWindow;
  }

  get available(): boolean {
    return typeof this.host.swiss?.request === "function";
  }

  async request<T = unknown>(path: string, options?: SwissRequestOptions): Promise<T> {
    const bridge = this.host.swiss;
    const request = bridge?.request;
    if (typeof request !== "function") {
      throw new Error("Swiss broker bridge is unavailable");
    }
    return request.call(bridge, path, options) as Promise<T>;
  }
}
