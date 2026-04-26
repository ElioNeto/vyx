import { AsyncLocalStorage } from 'node:async_hooks';

export interface RequestStore {
  correlationId: string;
}

export const requestContext = new AsyncLocalStorage<RequestStore>();

export function getCorrelationId(): string | undefined {
  return requestContext.getStore()?.correlationId;
}

export function runInRequestContext<T>(
  correlationId: string,
  fn: () => T
): T {
  return requestContext.run({ correlationId }, fn);
}

export async function runInRequestContextAsync<T>(
  correlationId: string,
  fn: () => Promise<T>
): Promise<T> {
  return requestContext.run({ correlationId }, fn);
}