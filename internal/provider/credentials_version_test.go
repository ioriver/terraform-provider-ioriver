package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func logDestListValue(items []LogDestinationModel) types.List {
	vals := make([]attr.Value, len(items))
	for i := range items {
		obj, diags := types.ObjectValueFrom(context.Background(), LogDestinationAttrTypes(), items[i])
		if diags.HasError() {
			return types.ListNull(types.ObjectType{AttrTypes: LogDestinationAttrTypes()})
		}
		vals[i] = obj
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: LogDestinationAttrTypes()}, vals)
}

func firstLogDest(t *testing.T, data *ServiceResourceModel) LogDestinationModel {
	t.Helper()
	var lds []LogDestinationModel
	if diags := data.Config.LogDestinations.ElementsAs(context.Background(), &lds, false); diags.HasError() {
		t.Fatalf("failed to decode log_destinations: %v", diags.Errors())
	}
	if len(lds) == 0 {
		t.Fatal("expected at least one log destination")
	}
	return lds[0]
}

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
			LogDestinations: logDestListValue([]LogDestinationModel{
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
			}),
		},
	}
}

func makeLogDestStateData(name string, credVer types.Int64) *ServiceResourceModel {
	// State never has credentials (WriteOnly), but does have credentials_version
	return &ServiceResourceModel{
		Config: &ServiceConfigModel{
			LogDestinations: logDestListValue([]LogDestinationModel{
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
			}),
		},
	}
}

// configData always provides credentials (user's HCL has them)
func makeLogDestConfigData(name string, credVer types.Int64) *ServiceResourceModel {
	return makeLogDestPlanData(name, credVer, true)
}

func makeCompatibleLogDestPlanData(name string, credVer types.Int64, withCreds bool) *ServiceResourceModel {
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
			LogDestinations: logDestListValue([]LogDestinationModel{
				{
					Name: types.StringValue(name),
					CompatibleS3: &EmbeddedCompatibleS3LogDestinationModel{
						Name:               types.StringValue("test-bucket"),
						Path:               types.StringValue("/"),
						Region:             types.StringValue("us-east-1"),
						Domain:             types.StringValue("storage.example.com"),
						Credentials:        creds,
						CredentialsVersion: credVer,
					},
				},
			}),
		},
	}
}

func makeCompatibleLogDestStateData(name string, credVer types.Int64) *ServiceResourceModel {
	return &ServiceResourceModel{
		Config: &ServiceConfigModel{
			LogDestinations: logDestListValue([]LogDestinationModel{
				{
					Name: types.StringValue(name),
					CompatibleS3: &EmbeddedCompatibleS3LogDestinationModel{
						Name:               types.StringValue("test-bucket"),
						Path:               types.StringValue("/"),
						Region:             types.StringValue("us-east-1"),
						Domain:             types.StringValue("storage.example.com"),
						Credentials:        nil,
						CredentialsVersion: credVer,
					},
				},
			}),
		},
	}
}

func makeCompatibleLogDestConfigData(name string, credVer types.Int64) *ServiceResourceModel {
	return makeCompatibleLogDestPlanData(name, credVer, true)
}

// ---------------------------------------------------------------------------
// Log destination: credentials_version tests
// ---------------------------------------------------------------------------

// On create: stateData is nil → credentials must be injected.
func TestCredentialsVersion_LogDest_Create_AlwaysSendCreds(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(1), false) // plan has no creds yet
	config := makeLogDestConfigData("dest1", types.Int64Value(1))

	mergeLogDestCredentialsFromConfig(plan, config, nil) // nil state = create

	ld := firstLogDest(t, plan)
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

	ld := firstLogDest(t, plan)
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

	ld := firstLogDest(t, plan)
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

	ld := firstLogDest(t, plan)
	if ld.AwsS3.Credentials == nil {
		t.Fatal("expected credentials to be injected post-import (state version is null), got nil")
	}
}

func TestCredentialsVersion_LogDest_Compatible_Create_AlwaysSendCreds(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeCompatibleLogDestConfigData("dest1", types.Int64Value(1))

	mergeLogDestCredentialsFromConfig(plan, config, nil)

	ld := firstLogDest(t, plan)
	if ld.CompatibleS3.Credentials == nil {
		t.Fatal("expected compatible_s3 credentials to be injected on create, got nil")
	}
}

func TestCredentialsVersion_LogDest_Compatible_Update_SameVersion_SkipCreds(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeCompatibleLogDestConfigData("dest1", types.Int64Value(1))
	state := makeCompatibleLogDestStateData("dest1", types.Int64Value(1))

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := firstLogDest(t, plan)
	if ld.CompatibleS3.Credentials != nil {
		t.Fatal("expected compatible_s3 credentials to be skipped when version unchanged, got non-nil")
	}
}

func TestCredentialsVersion_LogDest_Compatible_Update_BumpedVersion_SendCreds(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(2), false)
	config := makeCompatibleLogDestConfigData("dest1", types.Int64Value(2))
	state := makeCompatibleLogDestStateData("dest1", types.Int64Value(1))

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := firstLogDest(t, plan)
	if ld.CompatibleS3.Credentials == nil {
		t.Fatal("expected compatible_s3 credentials to be injected when version bumped, got nil")
	}
}

func TestCredentialsVersion_LogDest_Compatible_PostImport_SendCreds(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeCompatibleLogDestConfigData("dest1", types.Int64Value(1))
	state := makeCompatibleLogDestStateData("dest1", types.Int64Null())

	mergeLogDestCredentialsFromConfig(plan, config, state)

	ld := firstLogDest(t, plan)
	if ld.CompatibleS3.Credentials == nil {
		t.Fatal("expected compatible_s3 credentials to be injected post-import, got nil")
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

func makeOriginServiceModelWithPrivacy(name string, isPrivate bool, awsKey, awsSecret string) *ServiceResourceModel {
	s3 := &S3OriginModel{
		Host:            types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite: types.BoolValue(false),
		IsPrivate:       types.BoolValue(isPrivate),
	}
	if isPrivate {
		s3.S3AwsRegion = types.StringValue("us-east-1")
		s3.S3BucketName = types.StringValue("my-bucket")
	} else {
		s3.S3AwsRegion = types.StringNull()
		s3.S3BucketName = types.StringNull()
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

func makeOriginServiceModelWithPrivacyVersion(name string, isPrivate bool, credVer types.Int64, awsKey, awsSecret string) *ServiceResourceModel {
	s3 := &S3OriginModel{
		Host:            types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite: types.BoolValue(false),
		IsPrivate:       types.BoolValue(isPrivate),
	}
	if isPrivate {
		s3.S3AwsRegion = types.StringValue("us-east-1")
		s3.S3BucketName = types.StringValue("my-bucket")
		s3.CredentialsVersion = credVer
	} else {
		s3.S3AwsRegion = types.StringNull()
		s3.S3BucketName = types.StringNull()
		s3.CredentialsVersion = types.Int64Null()
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

	ld := firstLogDest(t, plan)
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

	ld := firstLogDest(t, plan)
	m := ld.ModelToMap()
	if _, ok := m["credentials"]; ok {
		t.Fatal("expected 'credentials' key to be absent in ModelToMap output when skipped")
	}
}

func TestCredentialsVersion_LogDest_Compatible_ModelToMap_CredsPresent(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(1), false)
	config := makeCompatibleLogDestConfigData("dest1", types.Int64Value(1))
	mergeLogDestCredentialsFromConfig(plan, config, nil)

	ld := firstLogDest(t, plan)
	m := ld.ModelToMap()
	if _, ok := m["credentials"]; !ok {
		t.Fatal("expected 'credentials' key in compatible_s3 ModelToMap output after inject")
	}
}

func TestLogDestination_CreateCredsEnforcement_AwsVersionWithoutCreds_Error(t *testing.T) {
	data := makeLogDestPlanData("dest1", types.Int64Value(1), false)

	if err := validateCreateLogDestinationCredentials(data); err == nil {
		t.Fatal("expected create-time error when aws_s3.credentials_version is set without credentials")
	}
}

func TestLogDestination_CreateCredsEnforcement_CompatibleVersionWithoutCreds_Error(t *testing.T) {
	data := makeCompatibleLogDestPlanData("dest1", types.Int64Value(1), false)

	if err := validateCreateLogDestinationCredentials(data); err == nil {
		t.Fatal("expected create-time error when compatible_s3.credentials_version is set without credentials")
	}
}

func TestLogDestination_UpdateCredsEnforcement_AwsVersionBumpedWithoutCreds_Error(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(2), false)
	state := makeLogDestStateData("dest1", types.Int64Value(1))

	if err := validateUpdateLogDestinationCredentials(plan, state); err == nil {
		t.Fatal("expected update-time error when aws_s3.credentials_version is bumped without credentials")
	}
}

func TestLogDestination_UpdateCredsEnforcement_CompatibleVersionBumpedWithoutCreds_Error(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Value(2), false)
	state := makeCompatibleLogDestStateData("dest1", types.Int64Value(1))

	if err := validateUpdateLogDestinationCredentials(plan, state); err == nil {
		t.Fatal("expected update-time error when compatible_s3.credentials_version is bumped without credentials")
	}
}

func TestLogDestination_UpdateCredsEnforcement_AwsVersionBumpedWithCreds_OK(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Value(2), true)
	state := makeLogDestStateData("dest1", types.Int64Value(1))

	if err := validateUpdateLogDestinationCredentials(plan, state); err != nil {
		t.Fatalf("expected update-time success when aws_s3.credentials_version is bumped with credentials, got: %v", err)
	}
}

func TestLogDestination_UpdateCredsEnforcement_AwsVersionOmitted_NoCreds_OK(t *testing.T) {
	plan := makeLogDestPlanData("dest1", types.Int64Null(), false)
	state := makeLogDestStateData("dest1", types.Int64Value(1))

	if err := validateUpdateLogDestinationCredentials(plan, state); err != nil {
		t.Fatalf("expected no error when aws_s3.credentials_version is omitted, got: %v", err)
	}
}

func TestLogDestination_UpdateCredsEnforcement_CompatibleVersionOmitted_NoCreds_OK(t *testing.T) {
	plan := makeCompatibleLogDestPlanData("dest1", types.Int64Null(), false)
	state := makeCompatibleLogDestStateData("dest1", types.Int64Value(1))

	if err := validateUpdateLogDestinationCredentials(plan, state); err != nil {
		t.Fatalf("expected no error when compatible_s3.credentials_version is omitted, got: %v", err)
	}
}

func TestLogDestination_ModelToMap_CredentialsPayloadEscapesJSON(t *testing.T) {
	ld := LogDestinationModel{
		Name:        types.StringValue("dest1"),
		AnonymizeIp: types.BoolValue(false),
		FileFormat:  types.StringValue("json-list"),
		CompatibleS3: &EmbeddedCompatibleS3LogDestinationModel{
			Name:               types.StringValue("bucket"),
			Path:               types.StringValue("/"),
			Region:             types.StringValue("us-east-1"),
			Domain:             types.StringValue("storage.example.com"),
			CredentialsVersion: types.Int64Value(1),
			Credentials: &AwsCredsModel{
				AssumeRole: &AwsAssumeRoleModel{
					RoleArn:    types.StringValue("arn:aws:iam::123:role/demo"),
					ExternalId: types.StringValue(`id-with-"quote"`),
				},
			},
		},
	}

	m := ld.ModelToMap()
	raw, ok := m["credentials"].(string)
	if !ok || raw == "" {
		t.Fatal("expected serialized credentials JSON payload")
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("expected credentials payload to be valid JSON, got error: %v", err)
	}
	if decoded["external_id"] != `id-with-"quote"` {
		t.Fatalf("expected external_id to round-trip correctly, got: %q", decoded["external_id"])
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateWithCreds_OK(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", true, "AKID", "SECRET")

	if err := validateCreatePrivateS3Credentials(data); err != nil {
		t.Fatalf("expected no error when private origin has credentials on create, got: %v", err)
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateMissingCreds_Error(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", true, "", "")

	if err := validateCreatePrivateS3Credentials(data); err == nil {
		t.Fatal("expected error when private origin has missing credentials on create")
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateMissingKeyOnly_Error(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", true, "", "SECRET")

	if err := validateCreatePrivateS3Credentials(data); err == nil {
		t.Fatal("expected error when private origin is missing s3_aws_key on create")
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateMissingSecretOnly_Error(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", true, "AKID", "")

	if err := validateCreatePrivateS3Credentials(data); err == nil {
		t.Fatal("expected error when private origin is missing s3_aws_secret on create")
	}
}

func TestCreatePrivateS3CredsEnforcement_PublicWithoutCreds_OK(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", false, "", "")

	if err := validateCreatePrivateS3Credentials(data); err != nil {
		t.Fatalf("expected no error when public origin omits credentials on create, got: %v", err)
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateUnknownCreds_OKAtPlan(t *testing.T) {
	s3 := &S3OriginModel{
		Host:               types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite:    types.BoolValue(false),
		IsPrivate:          types.BoolValue(true),
		S3AwsRegion:        types.StringValue("us-east-1"),
		S3BucketName:       types.StringValue("my-bucket"),
		S3AwsKey:           types.StringUnknown(),
		S3AwsSecret:        types.StringUnknown(),
		CredentialsVersion: types.Int64Value(1),
	}
	origin := OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue("origin1"),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		Shield:      nil,
		S3Origin:    s3,
	}
	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	list, _ := types.ListValueFrom(context.TODO(), elemType, []OriginModel{origin})
	data := &ServiceResourceModel{Config: &ServiceConfigModel{Origins: list}}

	if err := validateCreatePrivateS3Credentials(data); err != nil {
		t.Fatalf("expected no error when private creds are unknown during plan, got: %v", err)
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateUnknownKeyMissingSecret_Error(t *testing.T) {
	s3 := &S3OriginModel{
		Host:               types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite:    types.BoolValue(false),
		IsPrivate:          types.BoolValue(true),
		S3AwsRegion:        types.StringValue("us-east-1"),
		S3BucketName:       types.StringValue("my-bucket"),
		S3AwsKey:           types.StringUnknown(),
		S3AwsSecret:        types.StringNull(),
		CredentialsVersion: types.Int64Value(1),
	}
	origin := OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue("origin1"),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		Shield:      nil,
		S3Origin:    s3,
	}
	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	list, _ := types.ListValueFrom(context.TODO(), elemType, []OriginModel{origin})
	data := &ServiceResourceModel{Config: &ServiceConfigModel{Origins: list}}

	if err := validateCreatePrivateS3Credentials(data); err == nil {
		t.Fatal("expected error when private key is unknown but secret is missing")
	}
}

func TestCreatePrivateS3CredsEnforcement_PrivateMissingKeyUnknownSecret_Error(t *testing.T) {
	s3 := &S3OriginModel{
		Host:               types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite:    types.BoolValue(false),
		IsPrivate:          types.BoolValue(true),
		S3AwsRegion:        types.StringValue("us-east-1"),
		S3BucketName:       types.StringValue("my-bucket"),
		S3AwsKey:           types.StringNull(),
		S3AwsSecret:        types.StringUnknown(),
		CredentialsVersion: types.Int64Value(1),
	}
	origin := OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue("origin1"),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		Shield:      nil,
		S3Origin:    s3,
	}
	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	list, _ := types.ListValueFrom(context.TODO(), elemType, []OriginModel{origin})
	data := &ServiceResourceModel{Config: &ServiceConfigModel{Origins: list}}

	if err := validateCreatePrivateS3Credentials(data); err == nil {
		t.Fatal("expected error when private key is missing but secret is unknown")
	}
}

func TestCredentialsVersion_Origin_Update_PublicToPrivate_SendCreds(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	config := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "AKID", "SECRET")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", false, types.Int64Value(1), "", "")

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	if origins[0].S3Origin.S3AwsKey.IsNull() || origins[0].S3Origin.S3AwsKey.ValueString() == "" {
		t.Fatal("expected s3_aws_key to be injected when origin changes public->private")
	}
}

func TestCredentialsVersion_Origin_Update_PublicToPrivate_NoCredsInConfig_Skip(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	config := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", false, types.Int64Value(1), "", "")

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	if !origins[0].S3Origin.S3AwsKey.IsNull() && origins[0].S3Origin.S3AwsKey.ValueString() != "" {
		t.Fatal("expected s3_aws_key to remain absent when config has no creds on public->private update")
	}
}

func TestSensitiveCreds_FullFlow_Create_PrivateSerializesCreds(t *testing.T) {
	data := makeOriginServiceModelWithPrivacy("origin1", true, "AKID", "SECRET")

	if err := validateCreatePrivateS3Credentials(data); err != nil {
		t.Fatalf("expected create validation to pass, got: %v", err)
	}

	var origins []OriginModel
	data.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	m, err := origins[0].ModelToMap()
	if err != nil {
		t.Fatalf("unexpected ModelToMap error: %v", err)
	}

	s3, ok := m["s3_origin"].(map[string]interface{})
	if !ok {
		t.Fatal("expected s3_origin to be present in serialized payload")
	}
	if _, ok := s3["s3_aws_key"]; !ok {
		t.Fatal("expected s3_aws_key to be serialized for create")
	}
	if _, ok := s3["s3_aws_secret"]; !ok {
		t.Fatal("expected s3_aws_secret to be serialized for create")
	}
}

func TestSensitiveCreds_FullFlow_Update_VersionBumpSerializesCreds(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "", "")
	config := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "AKID_NEW", "SECRET_NEW")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	m, err := origins[0].ModelToMap()
	if err != nil {
		t.Fatalf("unexpected ModelToMap error: %v", err)
	}

	s3, ok := m["s3_origin"].(map[string]interface{})
	if !ok {
		t.Fatal("expected s3_origin to be present in serialized payload")
	}
	if _, ok := s3["s3_aws_key"]; !ok {
		t.Fatal("expected s3_aws_key to be serialized after version bump")
	}
	if _, ok := s3["s3_aws_secret"]; !ok {
		t.Fatal("expected s3_aws_secret to be serialized after version bump")
	}
}

func TestSensitiveCreds_FullFlow_Update_SameVersionSkipsSerialization(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	config := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "AKID", "SECRET")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	mergeS3OriginCredentialsFromConfig(plan, config, state)

	var origins []OriginModel
	plan.Config.Origins.ElementsAs(context.TODO(), &origins, false)
	m, err := origins[0].ModelToMap()
	if err != nil {
		t.Fatalf("unexpected ModelToMap error: %v", err)
	}

	s3, ok := m["s3_origin"].(map[string]interface{})
	if !ok {
		t.Fatal("expected s3_origin to be present in serialized payload")
	}
	if _, ok := s3["s3_aws_key"]; ok {
		t.Fatal("expected s3_aws_key to be omitted when version is unchanged")
	}
	if _, ok := s3["s3_aws_secret"]; ok {
		t.Fatal("expected s3_aws_secret to be omitted when version is unchanged")
	}
}

// ---------------------------------------------------------------------------
// Update-time credential enforcement (validateUpdatePrivateS3Credentials)
// ---------------------------------------------------------------------------

// Rotating credentials_version without providing new key/secret in config must fail
// during planning. State never carries the WriteOnly values, so a rotation with an
// empty config is a silent no-op otherwise.
func TestUpdatePrivateS3CredsEnforcement_VersionBumpedWithoutCreds_Error(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "", "")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err == nil {
		t.Fatal("expected error when credentials_version is bumped but creds are missing in config")
	}
}

// Same origin, same version, no creds in config → nothing to enforce.
func TestUpdatePrivateS3CredsEnforcement_SameVersionNoCreds_OK(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err != nil {
		t.Fatalf("expected no error when nothing rotated, got: %v", err)
	}
}

// Rotating credentials_version WITH both creds in config is valid.
func TestUpdatePrivateS3CredsEnforcement_VersionBumpedWithBothCreds_OK(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "AKID_NEW", "SECRET_NEW")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err != nil {
		t.Fatalf("expected rotation with both creds to be valid, got: %v", err)
	}
}

// Rotating credentials_version with only s3_aws_key present must fail.
func TestUpdatePrivateS3CredsEnforcement_VersionBumpedMissingSecret_Error(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "AKID_NEW", "")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err == nil {
		t.Fatal("expected error when only s3_aws_key is set during rotation")
	}
}

// Rotating credentials_version with only s3_aws_secret present must fail.
func TestUpdatePrivateS3CredsEnforcement_VersionBumpedMissingKey_Error(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(2), "", "SECRET_NEW")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err == nil {
		t.Fatal("expected error when only s3_aws_secret is set during rotation")
	}
}

// Flipping an existing origin from public to private WITHOUT creds in config must fail.
func TestUpdatePrivateS3CredsEnforcement_PublicToPrivateWithoutCreds_Error(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", false, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err == nil {
		t.Fatal("expected error when origin flips public->private without creds in config")
	}
}

// Flipping public to private WITH creds is valid, no version bump required
// (becamePrivate takes precedence over the version-change branch).
func TestUpdatePrivateS3CredsEnforcement_PublicToPrivateWithCreds_OK(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "AKID", "SECRET")
	state := makeOriginServiceModelWithPrivacyVersion("origin1", false, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err != nil {
		t.Fatalf("expected public->private with creds to be valid, got: %v", err)
	}
}

// Adding a brand-new private S3 origin as part of an update must apply the same
// create-time rule: both key and secret must be present in config.
func TestUpdatePrivateS3CredsEnforcement_NewPrivateOriginMissingCreds_Error(t *testing.T) {
	plan := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")
	// State has no origins at all (some other, non-S3 change is what triggered the update).
	state := &ServiceResourceModel{Config: &ServiceConfigModel{}}

	if err := validateUpdatePrivateS3Credentials(plan, state); err == nil {
		t.Fatal("expected error when a new private S3 origin is added without creds during update")
	}
}

// Deferring pair-unknown rotation is safe: both key AND secret unknown means the
// values will resolve at apply — plan phase must not error.
func TestUpdatePrivateS3CredsEnforcement_VersionBumpedBothCredsUnknown_OK(t *testing.T) {
	s3 := &S3OriginModel{
		Host:               types.StringValue("my-bucket.s3.amazonaws.com"),
		IsStaticWebsite:    types.BoolValue(false),
		IsPrivate:          types.BoolValue(true),
		S3AwsRegion:        types.StringValue("us-east-1"),
		S3BucketName:       types.StringValue("my-bucket"),
		S3AwsKey:           types.StringUnknown(),
		S3AwsSecret:        types.StringUnknown(),
		CredentialsVersion: types.Int64Value(2),
	}
	origin := OriginModel{
		Uuid:        types.StringNull(),
		Name:        types.StringValue("origin1"),
		Path:        types.StringValue("/"),
		VerifySSL:   types.BoolValue(true),
		TimeoutMs:   types.Int64Null(),
		SNIHostname: types.StringNull(),
		Shield:      nil,
		S3Origin:    s3,
	}
	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	list, _ := types.ListValueFrom(context.TODO(), elemType, []OriginModel{origin})
	plan := &ServiceResourceModel{Config: &ServiceConfigModel{Origins: list}}

	state := makeOriginServiceModelWithPrivacyVersion("origin1", true, types.Int64Value(1), "", "")

	if err := validateUpdatePrivateS3Credentials(plan, state); err != nil {
		t.Fatalf("expected pair-unknown rotation to be deferred to apply, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sensitive-log redaction (defense in depth)
// ---------------------------------------------------------------------------

func TestRedactedOriginsForLog_MasksAwsCreds(t *testing.T) {
	origins := []interface{}{
		map[string]interface{}{
			"path": "/",
			"s3_origin": map[string]interface{}{
				"host":          "my-bucket.s3.amazonaws.com",
				"is_private":    true,
				"s3_aws_key":    "AKIA_LEAKED",
				"s3_aws_secret": "SECRET_LEAKED",
			},
		},
	}

	s := redactedOriginsForLog(origins)
	if s == "" {
		t.Fatal("expected redacted output, got empty string")
	}
	if containsSubstring(s, "AKIA_LEAKED") {
		t.Fatal("s3_aws_key value leaked into log output")
	}
	if containsSubstring(s, "SECRET_LEAKED") {
		t.Fatal("s3_aws_secret value leaked into log output")
	}
	if !containsSubstring(s, "REDACTED") {
		t.Fatal("expected redaction marker in output")
	}

	// Original slice must not have been mutated.
	s3Orig, _ := origins[0].(map[string]interface{})["s3_origin"].(map[string]interface{})
	if s3Orig["s3_aws_key"] != "AKIA_LEAKED" {
		t.Fatal("redactedOriginsForLog mutated the original origins slice")
	}
}

func TestRedactedLogDestinationsForLog_MasksCredentials(t *testing.T) {
	logDests := []interface{}{
		map[string]interface{}{
			"name":        "dest1",
			"credentials": `{"access_key":"AKIA_LEAKED","secret_key":"SECRET_LEAKED"}`,
		},
	}

	s := redactedLogDestinationsForLog(logDests)
	if containsSubstring(s, "AKIA_LEAKED") || containsSubstring(s, "SECRET_LEAKED") {
		t.Fatal("log-destination credentials leaked into log output")
	}
	if !containsSubstring(s, "REDACTED") {
		t.Fatal("expected redaction marker in output")
	}

	// Original slice must not have been mutated.
	if logDests[0].(map[string]interface{})["credentials"] != `{"access_key":"AKIA_LEAKED","secret_key":"SECRET_LEAKED"}` {
		t.Fatal("redactedLogDestinationsForLog mutated the original log-destinations slice")
	}
}

// containsSubstring is a tiny substring check to avoid pulling in strings in tests
// that already have plenty of imports.
func containsSubstring(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
