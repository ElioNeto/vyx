import { get as g, post as p, put as u, patch as pa, start as s, del } from './dispatch.js';
import { getCorrelationId as gcid, requestContext, runInRequestContext, runInRequestContextAsync } from './context.js';
import { createResponse as cr, json as j, text as t, error as e } from './request.js';

export {
  g as get,
  p as post,
  u as put,
  del as delete,
  pa as patch,
  s as start,
};

export {
  cr as createResponse,
  j as json,
  t as text,
  e as error,
};

export { gcid as getCorrelationId, requestContext, runInRequestContext, runInRequestContextAsync };
export type { RequestStore } from './context.js';

export type { IPCPayload, Claims, WorkerResponse, Request, Response } from './request.js';
export type { WorkerOptions } from './dispatch.js';

export const worker = {
  get: g,
  post: p,
  put: u,
  delete: del,
  patch: pa,
  start: s,
};

export const logger = {
  info: (message: string, data?: Record<string, unknown>) => {
    const reqId = gcid();
    console.log(
      JSON.stringify({
        level: 'info',
        req_id: reqId,
        message,
        ...data,
        timestamp: new Date().toISOString(),
      })
    );
  },
  error: (message: string, data?: Record<string, unknown>) => {
    const reqId = gcid();
    console.error(
      JSON.stringify({
        level: 'error',
        req_id: reqId,
        message,
        ...data,
        timestamp: new Date().toISOString(),
      })
    );
  },
  warn: (message: string, data?: Record<string, unknown>) => {
    const reqId = gcid();
    console.warn(
      JSON.stringify({
        level: 'warn',
        req_id: reqId,
        message,
        ...data,
        timestamp: new Date().toISOString(),
      })
    );
  },
  debug: (message: string, data?: Record<string, unknown>) => {
    const reqId = gcid();
    console.log(
      JSON.stringify({
        level: 'debug',
        req_id: reqId,
        message,
        ...data,
        timestamp: new Date().toISOString(),
      })
    );
  },
};