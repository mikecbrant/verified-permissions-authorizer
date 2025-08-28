import { VerifiedPermissionsClient } from '@aws-sdk/client-verifiedpermissions';

let client: VerifiedPermissionsClient | undefined;

const getVerifiedPermissionsClient = (): VerifiedPermissionsClient => {
  if (!client) {
    client = new VerifiedPermissionsClient({});
  }
  return client;
};

export { getVerifiedPermissionsClient };
