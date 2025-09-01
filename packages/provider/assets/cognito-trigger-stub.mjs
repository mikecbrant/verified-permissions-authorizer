// Minimal, generic Cognito trigger stub used by the provider when no custom code is supplied.
// It accepts any event, logs it, and returns a permissive response shape for common triggers.
// This is intentionally generic; replace with project-specific logic by providing your own handlers.

export async function handler(event, context) {
  console.log('Cognito trigger event', JSON.stringify(event))

  // Best-effort shaping based on event triggerSource
  switch (event?.triggerSource) {
    case 'CreateAuthChallenge_Authentication':
      return {
        ...event,
        response: { publicChallengeParameters: {}, privateChallengeParameters: {}, challengeMetadata: 'STUB' },
      }
    case 'DefineAuthChallenge_Authentication':
      return { ...event, response: { challengeName: 'CUSTOM_CHALLENGE', issueTokens: true, failAuthentication: false } }
    case 'VerifyAuthChallengeResponse_Authentication':
      return { ...event, response: { answerCorrect: true } }
    case 'PreSignUp_SignUp':
    case 'PreSignUp_ExternalProvider':
      return { ...event, response: { autoConfirmUser: true, autoVerifyEmail: true, autoVerifyPhone: true } }
    case 'PostAuthentication_Authentication':
      return event
    case 'UserMigration_Authentication':
      return { ...event, response: { userAttributes: event?.userAttributes ?? {}, finalUserStatus: 'CONFIRMED', messageAction: 'SUPPRESS' } }
    case 'PreTokenGeneration_Authentication':
    case 'PreTokenGeneration_Authentication_Authentication':
      return event
    default:
      return event
  }
}
