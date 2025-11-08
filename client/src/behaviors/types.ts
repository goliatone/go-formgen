export interface BehaviorContext {
  element: HTMLElement;
  name: string;
  root: HTMLElement;
  config?: unknown;
}

export type BehaviorTeardown = void | (() => void) | { dispose(): void };

export type BehaviorFactory = (context: BehaviorContext) => BehaviorTeardown;
