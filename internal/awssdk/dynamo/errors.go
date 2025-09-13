package dynamo

// Note: Generic AWS/smithy error categories and classifiers live under
// internal/awssdk/errors. DynamoDB-specific helpers should use the common
// classifier and only add table/operation–specific details when necessary.
//
// Project logging standard: emit logs as message + structured context
// (logging.Fields). Avoid printf-style formatting and prefer JSON-friendly
// key/value context so logs compose cleanly across call stacks.
