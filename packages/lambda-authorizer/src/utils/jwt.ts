import type {
  APIGatewayRequestAuthorizerEvent,
  AppSyncAuthorizerEvent,
} from 'aws-lambda';
import { isApiGatewayRequestAuthorizerEvent, isAppSyncAuthorizerEvent } from './events.js';

type JwtPayload = Record<string, unknown> & {
  exp?: number;
  nbf?: number;
  iat?: number;
  sub?: string;
};

const base64UrlDecode = (segment: string): string => {
  const normalized = segment.replace(/-/g, '+').replace(/_/g, '/');
  const pad = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4));
  return Buffer.from(normalized + pad, 'base64').toString('utf8');
};

const parseJwtPayload = (token: string): JwtPayload | undefined => {
  const parts = token.split('.');
  if (parts.length !== 3) return undefined;
  try {
    const decoded = base64UrlDecode(parts[1] ?? '');
    const obj = JSON.parse(decoded) as JwtPayload;
    // basic temporal validation if fields present
    const now = Math.floor(Date.now() / 1000);
    if (typeof obj.nbf === 'number' && now < obj.nbf) return undefined;
    if (typeof obj.exp === 'number' && now >= obj.exp) return undefined;
    return obj;
  } catch {
    return undefined;
  }
};

const getBearerToken = (
  event: APIGatewayRequestAuthorizerEvent | AppSyncAuthorizerEvent,
): string | undefined => {
  if (isApiGatewayRequestAuthorizerEvent(event)) {
    const header = event.headers?.authorization ?? event.headers?.Authorization;
    if (!header) return undefined;
    const m = /^Bearer\s+(.+)$/i.exec(header.trim());
    return m?.[1];
  }
  if (isAppSyncAuthorizerEvent(event)) {
    const raw = event.authorizationToken?.trim();
    if (!raw) return undefined;
    const m = /^Bearer\s+(.+)$/i.exec(raw) || [undefined, raw];
    return m?.[1];
  }
  return undefined;
};

export type { JwtPayload };
export { parseJwtPayload, getBearerToken };
