import { createResponse, json, text, error, getCorrelationId } from '../src/request.js';
import * as context from '../src/context.js';
import { describe, it, expect, jest } from '@jest/globals';

describe('request helpers', () => {
  it('createResponse includes correlation id', () => {
    // mock correlation id
    jest.spyOn(context, 'getCorrelationId').mockReturnValue('corr-123');
    const resp = createResponse(200, { ok: true }, { headers: { 'X-Test': '1' } });
    expect(resp.status_code).toBe(200);
    expect(resp.body).toEqual({ ok: true });
    expect(resp.headers).toEqual({ 'X-Test': '1' });
    expect(resp.correlation_id).toBe('corr-123');
  });

  it('json helper sets content‑type', () => {
    jest.spyOn(context, 'getCorrelationId').mockReturnValue('cid');
    const resp = json({ msg: 'hi' }, 201);
    expect(resp.status_code).toBe(201);
    expect(resp.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(resp.body).toEqual({ msg: 'hi' });
    expect(resp.correlation_id).toBe('cid');
  });

  it('text helper sets content‑type', () => {
    jest.spyOn(context, 'getCorrelationId').mockReturnValue('cid2');
    const resp = text('plain', 202);
    expect(resp.status_code).toBe(202);
    expect(resp.headers).toEqual({ 'Content-Type': 'text/plain' });
    expect(resp.body).toBe('plain');
    expect(resp.correlation_id).toBe('cid2');
  });

  it('error helper returns JSON error payload', () => {
    jest.spyOn(context, 'getCorrelationId').mockReturnValue('cid3');
    const resp = error('boom', 400);
    expect(resp.status_code).toBe(400);
    expect(resp.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(resp.body).toEqual({ error: 'boom' });
    expect(resp.correlation_id).toBe('cid3');
  });
});
