import type { GetPublicKeyOrSecret, Secret } from 'jsonwebtoken'

import { type JwtPayload, parseJwtPayload } from '../utils/jwt.js'

const authenticate = (
  token: string,
  key: Secret | GetPublicKeyOrSecret,
): JwtPayload => {
  const payload = parseJwtPayload(token, key)
  if (!payload) throw new Error('Invalid or expired JWT')
  return payload
}

export { authenticate }
