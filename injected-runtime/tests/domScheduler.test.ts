import test from "node:test";
import assert from "node:assert/strict";
import { DomScheduler } from "../src/core/domScheduler";

function element(matchesSelector: (selector: string) => boolean, query: (selector: string) => Element[] = () => []): Element {
  return {
    nodeType: 1,
    parentElement: null,
    matches: matchesSelector,
    querySelectorAll: query,
  } as unknown as Element;
}

function createDocument(buttons: Element[]) {
  let visibility: DocumentVisibilityState = "visible";
  const visibilityListeners: EventListener[] = [];
  const body = element(() => false, (selector) => selector === "button" ? buttons : []);
  const documentValue = {
    body,
    defaultView: null,
    get visibilityState() {
      return visibility;
    },
    addEventListener(type: string, listener: EventListener) {
      if (type === "visibilitychange") visibilityListeners.push(listener);
    },
    removeEventListener(type: string, listener: EventListener) {
      if (type !== "visibilitychange") return;
      const index = visibilityListeners.indexOf(listener);
      if (index >= 0) visibilityListeners.splice(index, 1);
    },
  } as unknown as Document;
  return {
    document: documentValue,
    body,
    setVisible(visible: boolean) {
      visibility = visible ? "visible" : "hidden";
      for (const listener of visibilityListeners) listener(new Event("visibilitychange"));
    },
  };
}

function mutation(target: Node, addedNodes: Node[]): MutationRecord {
  return { target, addedNodes } as unknown as MutationRecord;
}

test("dom scheduler batches selector routes into one frame", () => {
  const first = element((selector) => selector === "button");
  const second = element((selector) => selector === "button");
  const fixture = createDocument([first, second]);
  const frames: FrameRequestCallback[] = [];
  const observers: Array<{ callback: MutationCallback; emit(records: MutationRecord[]): void }> = [];
  const scheduler = new DomScheduler({
    document: fixture.document,
    requestFrame(callback) {
      frames.push(callback);
      return frames.length;
    },
    cancelFrame() {},
    observerFactory(callback) {
      const observer = {
        callback,
        observe() {},
        disconnect() {},
        takeRecords() {
          return [];
        },
        emit(records: MutationRecord[]) {
          callback(records, observer as unknown as MutationObserver);
        },
      };
      observers.push(observer);
      return observer as unknown as MutationObserver;
    },
  });
  const seen: Element[] = [];
  scheduler.on("button", (matched) => seen.push(matched));

  assert.equal(scheduler.start(), true);
  assert.equal(scheduler.observerCount, 1);
  observers[0]!.emit([mutation(fixture.body, [first])]);
  observers[0]!.emit([mutation(fixture.body, [second])]);
  assert.equal(frames.length, 1);
  assert.equal(seen.length, 0);

  frames.shift()!(0);
  assert.deepEqual(seen, [first, second]);
  scheduler.stop();
});

test("dom scheduler holds mutation routes while the document is hidden", () => {
  const button = element((selector) => selector === "button");
  const fixture = createDocument([button]);
  const frames: FrameRequestCallback[] = [];
  const observers: Array<{ emit(records: MutationRecord[]): void }> = [];
  const scheduler = new DomScheduler({
    document: fixture.document,
    requestFrame(callback) {
      frames.push(callback);
      return frames.length;
    },
    cancelFrame() {},
    observerFactory(callback) {
      const observer = {
        observe() {},
        disconnect() {},
        takeRecords() {
          return [];
        },
        emit(records: MutationRecord[]) {
          callback(records, observer as unknown as MutationObserver);
        },
      };
      observers.push(observer);
      return observer as unknown as MutationObserver;
    },
  });
  let calls = 0;
  scheduler.on("button", () => { calls += 1; });
  scheduler.start();

  fixture.setVisible(false);
  observers[0]!.emit([mutation(fixture.body, [button])]);
  assert.equal(frames.length, 0);
  assert.equal(calls, 0);

  fixture.setVisible(true);
  assert.equal(frames.length, 1);
  frames.shift()!(0);
  assert.equal(calls, 1);
  scheduler.stop();
});
