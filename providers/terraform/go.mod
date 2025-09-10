module github.com/mikecbrant/verified-permissions-authorizer/providers/terraform

go 1.24

require (
    github.com/hashicorp/terraform-plugin-framework v1.10.0
    github.com/hashicorp/terraform-plugin-testing v1.6.0
    github.com/aws/aws-sdk-go-v2/config v1.27.16
    github.com/aws/aws-sdk-go-v2/service/iam v1.51.0
    github.com/aws/aws-sdk-go-v2/service/lambda v1.65.0
    github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.38.0
    github.com/aws/aws-sdk-go-v2/service/dynamodb v1.37.1
    github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider v1.54.0
    github.com/aws/aws-sdk-go-v2/service/verifiedpermissions v1.14.1
    github.com/mikecbrant/verified-permissions-authorizer/providers/internal v0.0.0
)

replace github.com/mikecbrant/verified-permissions-authorizer/providers/internal => ../internal
