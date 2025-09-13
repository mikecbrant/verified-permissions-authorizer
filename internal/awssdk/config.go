package awssdk

import (
    "context"

    awsv2 "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

// LoadDefault loads the default AWS configuration for the given region using the
// standard environment/credentials chain.
func LoadDefault(ctx context.Context, region string) (awsv2.Config, error) {
    if region == "" {
        return awsconfig.LoadDefaultConfig(ctx)
    }
    return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
}

// PartitionForRegion derives the AWS partition from a region name.
func PartitionForRegion(region string) string {
    switch {
    case len(region) >= 3 && region[:3] == "cn-":
        return "aws-cn"
    case len(region) >= 7 && region[:7] == "us-gov-":
        return "aws-us-gov"
    default:
        return "aws"
    }
}
