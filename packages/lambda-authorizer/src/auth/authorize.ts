import type { IsAuthorizedInput } from '@aws-sdk/client-verifiedpermissions'

import { isAuthorized } from '../aws/verified-permissions.js'

const authorize = async (input: IsAuthorizedInput): Promise<boolean> => {
  const decision = await isAuthorized(input)
  return decision === 'ALLOW'
}

export { authorize }
