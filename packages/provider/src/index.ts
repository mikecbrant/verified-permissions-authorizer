import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import * as awsIam from '@pulumi/aws/iam';
import * as awsLambda from '@pulumi/aws/lambda';
import * as awsLogs from '@pulumi/aws/cloudwatch';
import * as awsVp from '@pulumi/aws/verifiedpermissions';
import * as path from 'node:path';
import * as fs from 'node:fs';

export interface AuthorizerWithPolicyStoreArgs {
  description?: pulumi.Input<string>;
  validationMode?: pulumi.Input<'STRICT' | 'OFF'>;
  runtime?: pulumi.Input<string>; // e.g., nodejs20.x
  lambdaEnvironment?: pulumi.Input<Record<string, pulumi.Input<string>>>;
  logRetentionDays?: pulumi.Input<number>;
}

export class AuthorizerWithPolicyStore extends pulumi.ComponentResource {
  readonly policyStoreId: pulumi.Output<string>;
  readonly policyStoreArn: pulumi.Output<string>;
  readonly functionArn: pulumi.Output<string>;
  readonly roleArn: pulumi.Output<string>;

  constructor(
    name: string,
    args: AuthorizerWithPolicyStoreArgs = {},
    opts?: pulumi.ComponentResourceOptions,
  ) {
    super('verified-permissions-authorizer:index:AuthorizerWithPolicyStore', name, {}, opts);

    const runtime = args.runtime ?? 'nodejs20.x';
    const validationMode = args.validationMode ?? 'STRICT';
    const description = args.description;
    const logRetentionDays = args.logRetentionDays ?? 14;

    // 1) Verified Permissions Policy Store
    const storeArgs: awsVp.PolicyStoreArgs = {
      validationSettings: { mode: validationMode },
      // tags could be exposed later
      ...(description !== undefined ? { description } : {}),
    };
    const store = new awsVp.PolicyStore(`${name}-store`, storeArgs, { parent: this });

    // 2) IAM Role for Lambda
    const role = new awsIam.Role(
      `${name}-role`,
      {
        assumeRolePolicy: aws.iam.assumeRolePolicyForPrincipal({ Service: 'lambda.amazonaws.com' }),
        description: 'Role for Verified Permissions Lambda Authorizer',
      },
      { parent: this },
    );

    new awsIam.RolePolicyAttachment(
      `${name}-logs`,
      {
        role: role.name,
        policyArn: aws.iam.ManagedPolicy.AWSLambdaBasicExecutionRole,
      },
      { parent: this },
    );

    const vpPolicy = new awsIam.Policy(
      `${name}-vp`,
      {
        policy: JSON.stringify({
          Version: '2012-10-17',
          Statement: [
            {
              Sid: 'GetPolicyStore',
              Effect: 'Allow',
              Action: ['verifiedpermissions:GetPolicyStore'],
              Resource: '*',
            },
          ],
        }),
      },
      { parent: this },
    );

    new awsIam.RolePolicyAttachment(
      `${name}-vp-attach`,
      {
        role: role.name,
        policyArn: vpPolicy.arn,
      },
      { parent: this },
    );

    // 3) Package bundled authorizer from the sibling package
    const lambdaPkgMain = require.resolve('verified-permissions-lambda-authorizer');
    const lambdaFile = path.resolve(lambdaPkgMain);
    const bundledFile = fs.existsSync(lambdaFile)
      ? lambdaFile
      : (() => {
          throw new Error('Bundled authorizer not found');
        })();

    const code = new pulumi.asset.AssetArchive({
      'index.js': new pulumi.asset.FileAsset(bundledFile),
    });

    const fn = new awsLambda.Function(
      `${name}-authorizer`,
      {
        role: role.arn,
        runtime,
        handler: 'index.handler',
        code,
        environment: {
          variables: {
            POLICY_STORE_ID: store.id,
            ...args.lambdaEnvironment,
          },
        },
        architectures: ['arm64'],
        timeout: 10,
      },
      { parent: this },
    );

    new awsLogs.LogGroup(
      `${name}-lg`,
      {
        name: pulumi.interpolate`/aws/lambda/${fn.name}`,
        retentionInDays: logRetentionDays,
      },
      { parent: this },
    );

    this.policyStoreId = store.id;
    this.policyStoreArn = store.arn;
    this.functionArn = fn.arn;
    this.roleArn = role.arn;

    this.registerOutputs({
      policyStoreId: this.policyStoreId,
      policyStoreArn: this.policyStoreArn,
      functionArn: this.functionArn,
      roleArn: this.roleArn,
    });
  }
}
