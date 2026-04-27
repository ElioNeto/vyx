import {
  get,
  post,
  put,
  patch,
  start,
  del,
} from './dispatch.js';

import {
  createResponse,
  json,
  text,
  error,
} from './request.js';

import {
  getCorrelationId,
  requestContext,
  runInRequestContext,
  runInRequestContextAsync,
} from './context.js';

export {
  get,
  post,
  put,
  patch,
  start,
  del as delete,
  createResponse,
  json,
  text,
  error,
  getCorrelationId,
  requestContext,
  runInRequestContext,
  runInRequestContextAsync,
};

export type { RequestStore } from './context.js';
export type { IPCPayload, Claims, WorkerResponse, Request, Response } from './request.js';
export type { WorkerOptions } from './dispatch.js';

export const worker = {
  get,
  post,
  put,
  delete: del,
  patch,
  start,
};

export const logger = {
  info: (message: string, data?: Record<string, unknown>) => {
    const reqId = getCorrelationId();
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
    const reqId = getCorrelationId();
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
    const reqId = getCorrelationId();
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
    const reqId = getCorrelationId();
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