import * as pulumi from '@pulumi/pulumi'

type ValidationMode = 'STRICT' | 'OFF'

type AuthorizerWithPolicyStoreArgs = {
  description?: pulumi.Input<string>
  validationMode?: pulumi.Input<ValidationMode>
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>
}

class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  constructor(name: string, args: AuthorizerWithPolicyStoreArgs, opts?: pulumi.ComponentResourceOptions) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, {}, opts)
  }
}

export { AuthorizerWithPolicyStore, type AuthorizerWithPolicyStoreArgs, type ValidationMode }
