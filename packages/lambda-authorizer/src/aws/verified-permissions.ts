import {
  IsAuthorizedCommand,
  type IsAuthorizedInput,
  VerifiedPermissionsClient,
} from '@aws-sdk/client-verifiedpermissions'

let client: VerifiedPermissionsClient | undefined

const getClient = (): VerifiedPermissionsClient => {
  if (!client) client = new VerifiedPermissionsClient({})
  return client
}

type Decision = 'ALLOW' | 'DENY'

const isAuthorized = async (input: IsAuthorizedInput): Promise<Decision> => {
  const c = getClient()
  const out = await c.send(new IsAuthorizedCommand(input))
  return (out.decision ?? 'DENY') as Decision
}

export type { Decision }
export { getClient, isAuthorized }
