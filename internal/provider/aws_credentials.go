package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

type AwsAccessKeyModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

type AwsAssumeRoleModel struct {
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
}

type AwsCredsModel struct {
	AccessKey  *AwsAccessKeyModel  `tfsdk:"access_key"`
	AssumeRole *AwsAssumeRoleModel `tfsdk:"assume_role"`
}
