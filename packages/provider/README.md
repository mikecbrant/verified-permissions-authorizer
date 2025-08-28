# pulumi-verified-permissions-authorizer

Pulumi component that creates an AWS Verified Permissions Policy Store and a bundled AWS Lambda Authorizer function that can be wired to API Gateway.

Outputs include the policy store identifiers and Lambda function/role ARNs.

Example (TypeScript):

```ts
import * as pulumi from '@pulumi/pulumi';
import { AuthorizerWithPolicyStore } from 'pulumi-verified-permissions-authorizer';

const stack = new AuthorizerWithPolicyStore('vp', {
  description: 'VP store for API auth',
  validationMode: 'STRICT',
  lambdaEnvironment: { EXTRA: '1' },
});

export const policyStoreId = stack.policyStoreId;
export const functionArn = stack.functionArn;
```

Notes
- The Lambda runtime is fixed to `nodejs22.x` and is not configurable. AWS region/credentials are inherited from the standard Pulumi AWS provider configuration.
```
