import { type JwtPayload,parseJwtPayload } from '../utils/jwt.js'

const authenticate = (token: string): JwtPayload => {
  const payload = parseJwtPayload(token)
  if (!payload) throw new Error('Invalid or expired JWT')
  return payload
}

export { authenticate }
