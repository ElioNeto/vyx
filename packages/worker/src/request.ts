import { getCorrelationId } from './context.js';

export interface IPCPayload {
  method: string;
  path: string;
  headers: Record<string, string>;
  query: Record<string, string>;
  params: Record<string, string>;
  body: unknown;
  claims: Claims | null;
  correlation_id: string;
}

export interface Claims {
  user_id: string;
  roles: string[];
}

export interface WorkerResponse {
  status_code: number;
  headers?: Record<string, string>;
  body?: unknown;
  correlation_id: string;
}

export interface Request {
  method: string;
  path: string;
  headers: Record<string, string>;
  query: Record<string, string>;
  params: Record<string, string>;
  body: unknown;
  claims: Claims | null;
}

export interface Response {
  statusCode: number;
  headers: Record<string, string>;
  body: unknown;
}

export function createResponse(
  statusCode: number,
  body: unknown,
  options?: { headers?: Record<string, string> }
): WorkerResponse {
  const correlationId = getCorrelationId();
  return {
    status_code: statusCode,
    headers: options?.headers,
    body,
    correlation_id: correlationId ?? '',
  };
}

export function json(obj: unknown, statusCode = 200): WorkerResponse {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  return createResponse(statusCode, obj, { headers });
}

export function text(body: string, statusCode = 200): WorkerResponse {
  const headers: Record<string, string> = { 'Content-Type': 'text/plain' };
  return createResponse(statusCode, body, { headers });
}

export function error(message: string, statusCode = 500): WorkerResponse {
  return json({ error: message }, statusCode);
}