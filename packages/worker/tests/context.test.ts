import { requestContext, getCorrelationId, runInRequestContext, runInRequestContextAsync } from '../src/context';

describe('AsyncLocalStorage context', () => {
  it('should return undefined when not in request context', () => {
    expect(getCorrelationId()).toBeUndefined();
  });

  it('should store and retrieve correlation ID', () => {
    const result = runInRequestContext('test-correlation-123', () => {
      return getCorrelationId();
    });
    expect(result).toBe('test-correlation-123');
  });

  it('should isolate context between concurrent requests', async () => {
    const promises: Promise<string>[] = [];

    for (let i = 0; i < 5; i++) {
      const correlationId = `correlation-${i}`;
      const p = runInRequestContextAsync(correlationId, async () => {
        await new Promise((resolve) => setTimeout(resolve, Math.random() * 10));
        return getCorrelationId()!;
      });
      promises.push(p);
    }

    const results = await Promise.all(promises);
    results.forEach((result, index) => {
      expect(result).toBe(`correlation-${index}`);
    });
  });

  it('should preserve context in nested async calls', async () => {
    const result = await runInRequestContextAsync('nested-correlation', async () => {
      const innerResult = await Promise.resolve().then(() => {
        return getCorrelationId();
      });
      return innerResult;
    });

    expect(result).toBe('nested-correlation');
  });

  it('should work with Promise.then chains', async () => {
    const result = await runInRequestContextAsync('promise-chain-id', async () => {
      return Promise.resolve()
        .then(() => getCorrelationId())
        .then((id) => id);
    });

    expect(result).toBe('promise-chain-id');
  });

  it('should not leak context after request completes', () => {
    runInRequestContext('should-not-leak', () => {
      expect(getCorrelationId()).toBe('should-not-leak');
    });

    expect(getCorrelationId()).toBeUndefined();
  });
});