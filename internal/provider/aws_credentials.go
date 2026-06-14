package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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

func AwsAccessKeyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"access_key": types.StringType,
		"secret_key": types.StringType,
	}
}

func AwsAssumeRoleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"role_arn":    types.StringType,
		"external_id": types.StringType,
	}
}

func AwsCredsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"access_key":  types.ObjectType{AttrTypes: AwsAccessKeyAttrTypes()},
		"assume_role": types.ObjectType{AttrTypes: AwsAssumeRoleAttrTypes()},
	}
}
