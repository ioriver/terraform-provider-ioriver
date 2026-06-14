package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeLogDestPlanData(name string, credVer types.Int64, withCreds bool) *ServiceResourceModel {
	creds := (*AwsCredsModel)(nil)
	if withCreds {
		creds = &AwsCredsModel{
			AccessKey: &AwsAccessKeyModel{
				AccessKey: types.StringValue("AKIAIOSFODNN7EXAMPLE"),
				SecretKey: types.StringValue("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
			},
		}
	}
	return &ServiceResourceModel{
		Config: &ServiceConfigModel{
			LogDestinations: &[]LogDestinationModel{
				{
					Name: types.StringValue(name),
					AwsS3: &EmbeddedAwsS3LogDestinationModel{
						Name:               types.StringValue("test-bucket"),
						Path:               types.StringValue("/"),
						Region:             types.StringValue("us-east-1"),
						Credentials:        creds,
						CredentialsVersion: credVer,
					},
				},
			},
		},
	}
}

func makeLogDestStateData(name string, credVer types.Int64) *ServiceResourceModel {
	// State never has credentials (WriteOnly), but does have credentials_version
	return &ServiceResourceModel{
		Config: &ServiceConfigModel{
			LogDestinations: &[]LogDestinationModel{
				{
					Name: types.StringValue(name),
					AwsS3: &EmbeddedAwsS3LogDestinationModel{
						Name:               types.StringValue("test-bucket"),
						Path:               types.StringValue("/"),
						Region:             types.StringValue("us-east-1"),
						Credentials:        nil, // never in state
						CredentialsVersion: credVer,
					},
				},
			},
		},
	}
}

// configData always provides credentials (user's HCL has them)
func makeLogDestConfigData(name string, credVer types.Int64) *ServiceResourceModel {
	return makeLogDestPlanData(name, credVer, true)
}

// ---------------------------------------------------------------------------
// Log destination: credentials_version tests
// ---------------------------------------------------------------------------

// On create: stateData is nil → credentials must be injected.
func TestCredentialsVersion_LogDest_Create_AlwaysSendCreds(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false) // plan has no creds yet
	config := makeLogDestConfigData("dest1", types.Int64Value(1))

	mergeLogDestCredentialsFromConfig(plan, config, nil) // nil state = create

	ld := (*plan.Config.LogDestinations)[0]
	if ld.AwsS3.Credentials == nil {
		t.Fatal("expected credentials to be injected on create, got nil")
	}
}

// On update with same version: credentials must NOT be injected.
func TestCredentialsVersion_LogDest_Update_SameVersion_SkipCreds(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeLogDestConfigData("dest1", types.Int64Value(1))
	state := makeLogDestStateData("dest1", types.Int64Value(1)) // same version

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := (*plan.Config.LogDestinations)[0]
	if ld.AwsS3.Credentials != nil {
		t.Fatal("expected credentials to be skipped when version unchanged, got non-nil")
	}
}

// On update with bumped version: credentials must be injected.
func TestCredentialsVersion_LogDest_Update_BumpedVersion_SendCreds(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(2), false) // bumped to 2
	config := makeLogDestConfigData("dest1", types.Int64Value(2))
	state := makeLogDestStateData("dest1", types.Int64Value(1)) // was 1

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := (*plan.Config.LogDestinations)[0]
	if ld.AwsS3.Credentials == nil {
		t.Fatal("expected credentials to be injected when version bumped, got nil")
	}
}

// Post-import first apply: state has null version → credentials must be injected.
func TestCredentialsVersion_LogDest_PostImport_SendCreds(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeLogDestConfigData("dest1", types.Int64Value(1))
	state := makeLogDestStateData("dest1", types.Int64Null()) // null after import

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := (*plan.Config.LogDestinations)[0]
	if ld.AwsS3.Credentials == nil {
		t.Fatal("expected credentials to be injected post-import (state version is null), got nil")
	}
}

// ---------------------------------------------------------------------------
// S3 Origin: credentials_version tests
// ---------------------------------------------------------------------------

func makeOriginServiceModel(name string, credVer types.Int64, awsKey, awsSecret string) *ServiceResourceModel {
	s3 := &S3OriginModel{
		Host:               types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite:    types.BoolValue(false),
		IsPrivate:          types.BoolValue(true),
		S3AwsRegion:        types.StringValue("us-east-1"),
		S3BucketName:       types.StringValue("my-bucket"),
		CredentialsVersion: credVer,
	}
	if awsKey != "" {
		s3.S3AwsKey = types.StringValue(awsKey)
		s3.S3AwsSecret = types.StringValue(awsSecret)
	} else {
		s3.S3AwsKey = types.StringNull()
		s3.S3AwsSecret = types.StringNull()
	}

	origin := OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue(name),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		Shield:      nil,
		S3Origin:    s3,
	}

	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	list, _ := types.ListValueFrom(context.TODO(), elemType, []OriginModel{origin})

	return &ServiceResourceModel{
		Config: &ServiceConfigModel{
			Origins: list,
		},
	}
}

// On create: stateData is nil → credentials must be injected.
func TestCredentialsVersion_Origin_Create_AlwaysSendCreds(t *testing.T) {
	plan := makeOriginServiceModel("origin1", types.Int64Value(1), "", "")
	config := makeOriginServiceModel("origin1", types.Int64Value(1), "AKID", "SECRET")

	mergeS3OriginCredentialsFromConfig(plan, config, nil)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	if origins[0].S3Origin.S3AwsKey.IsNull() || origins[0].S3Origin.S3AwsKey.ValueString() == "" {
		t.Fatal("expected s3_aws_key to be injected on create, got empty/null")
	}
}

// On update with same version: credentials must NOT be injected.
func TestCredentialsVersion_Origin_Update_SameVersion_SkipCreds(t *testing.T) {
	plan := makeOriginServiceModel("origin1", types.Int64Value(1), "", "")
	config := makeOriginServiceModel("origin1", types.Int64Value(1), "AKID", "SECRET")
	state := makeOriginServiceModel("origin1", types.Int64Value(1), "", "") // same version

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	if !origins[0].S3Origin.S3AwsKey.IsNull() && origins[0].S3Origin.S3AwsKey.ValueString() != "" {
		t.Fatal("expected s3_aws_key to be skipped when version unchanged")
	}
}

// On update with bumped version: credentials must be injected.
func TestCredentialsVersion_Origin_Update_BumpedVersion_SendCreds(t *testing.T) {
	plan := makeOriginServiceModel("origin1", types.Int64Value(2), "", "")
	config := makeOriginServiceModel("origin1", types.Int64Value(2), "AKID_NEW", "SECRET_NEW")
	state := makeOriginServiceModel("origin1", types.Int64Value(1), "", "") // was 1

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(t.Context(), &origins, false)
	if origins[0].S3Origin.S3AwsKey.IsNull() || origins[0].S3Origin.S3AwsKey.ValueString() == "" {
		t.Fatal("expected s3_aws_key to be injected when version bumped, got empty/null")
	}
}

// Post-import first apply: state has null version → credentials must be injected.
func TestCredentialsVersion_Origin_PostImport_SendCreds(t *testing.T) {
	plan := makeOriginServiceModel("origin1", types.Int64Value(1), "", "")
	config := makeOriginServiceModel("origin1", types.Int64Value(1), "AKID", "SECRET")
	state := makeOriginServiceModel("origin1", types.Int64Null(), "", "") // null after import

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(t.Context(), &origins, false)
	if origins[0].S3Origin.S3AwsKey.IsNull() || origins[0].S3Origin.S3AwsKey.ValueString() == "" {
		t.Fatal("expected s3_aws_key to be injected post-import (state version is null), got empty/null")
	}
}

// ---------------------------------------------------------------------------
// Log destination: ModelToMap reflects inject/skip correctly
// ---------------------------------------------------------------------------

func TestCredentialsVersion_LogDest_ModelToMap_CredsPresent(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeLogDestConfigData("dest1", types.Int64Value(1))
	mergeLogDestCredentialsFromConfig(plan, config, nil) // create → inject

	ld := (*plan.Config.LogDestinations)[0]
	m := ld.ModelToMap()
	if _, ok := m["credentials"]; !ok {
		t.Fatal("expected 'credentials' key in ModelToMap output after inject")
	}
}

func TestCredentialsVersion_LogDest_ModelToMap_CredsAbsent(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeLogDestConfigData("dest1", types.Int64Value(1))
	state := makeLogDestStateData("dest1", types.Int64Value(1)) // same version → skip
	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := (*plan.Config.LogDestinations)[0]
	m := ld.ModelToMap()
	if _, ok := m["credentials"]; ok {
		t.Fatal("expected 'credentials' key to be absent in ModelToMap output when skipped")
	}
}
