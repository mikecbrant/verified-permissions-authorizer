module github.com/mikecbrant/verified-permissions-authorizer

go 1.25

require (
	github.com/aws/aws-sdk-go-v2/config v1.27.16
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.37.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.65.0
	github.com/aws/aws-sdk-go-v2/service/verifiedpermissions v1.14.1
	github.com/bmatcuk/doublestar/v4 v4.7.1
	github.com/hashicorp/terraform-plugin-framework v1.10.0
	github.com/hashicorp/terraform-plugin-testing v1.6.0
	github.com/pulumi/pulumi-aws/sdk/v6 v6.73.0
	github.com/pulumi/pulumi-go-provider v1.1.1
	github.com/pulumi/pulumi/sdk/v3 v3.162.0
	gopkg.in/yaml.v3 v3.0.1
)
