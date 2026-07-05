package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// EmbeddedAwsS3LogDestinationModel is used when log destinations are embedded
// inside a service resource. Credentials is a pointer so the framework can
// represent null (user omitted the optional write-only block).
type EmbeddedAwsS3LogDestinationModel struct {
	Name               types.String   `tfsdk:"name"`
	Path               types.String   `tfsdk:"path"`
	Region             types.String   `tfsdk:"region"`
	Credentials        *AwsCredsModel `tfsdk:"credentials"`
	CredentialsVersion types.Int64    `tfsdk:"credentials_version"` // TF-only counter; increment to push new credentials
}

// EmbeddedCompatibleS3LogDestinationModel is the same idea for compatible-S3.
type EmbeddedCompatibleS3LogDestinationModel struct {
	Name               types.String   `tfsdk:"name"`
	Path               types.String   `tfsdk:"path"`
	Region             types.String   `tfsdk:"region"`
	Domain             types.String   `tfsdk:"domain"`
	Credentials        *AwsCredsModel `tfsdk:"credentials"`
	CredentialsVersion types.Int64    `tfsdk:"credentials_version"` // TF-only counter; increment to push new credentials
}

type LogDestinationModel struct {
	Name         types.String                             `tfsdk:"name"`
	Uuid         types.String                             `tfsdk:"uuid"`
	AnonymizeIp  types.Bool                               `tfsdk:"anonymize_ip"`
	FileFormat   types.String                             `tfsdk:"file_format"`
	AwsS3        *EmbeddedAwsS3LogDestinationModel        `tfsdk:"aws_s3"`
	CompatibleS3 *EmbeddedCompatibleS3LogDestinationModel `tfsdk:"compatible_s3"`
}

func (l LogDestinationModel) GetName() string {
	return l.Name.ValueString()
}

func EmbeddedAwsS3AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                types.StringType,
		"path":                types.StringType,
		"region":              types.StringType,
		"credentials":         types.ObjectType{AttrTypes: AwsCredsAttrTypes()},
		"credentials_version": types.Int64Type,
	}
}

func EmbeddedCompatibleS3AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                types.StringType,
		"path":                types.StringType,
		"region":              types.StringType,
		"domain":              types.StringType,
		"credentials":         types.ObjectType{AttrTypes: AwsCredsAttrTypes()},
		"credentials_version": types.Int64Type,
	}
}

func LogDestinationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":          types.StringType,
		"uuid":          types.StringType,
		"anonymize_ip":  types.BoolType,
		"file_format":   types.StringType,
		"aws_s3":        types.ObjectType{AttrTypes: EmbeddedAwsS3AttrTypes()},
		"compatible_s3": types.ObjectType{AttrTypes: EmbeddedCompatibleS3AttrTypes()},
	}
}

func LogDestinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Log destination name",
			Required:            true,
		},
		"uuid": schema.StringAttribute{
			MarkdownDescription: "Origin UUID (managed by system)",
			Computed:            true,
			// We Do NOT use UseStateForUnknown() here.
			// The NamedListPlanModifier resolves uuid from state
		},
		"anonymize_ip": schema.BoolAttribute{
			MarkdownDescription: "Whether to anonymize IP addresses in logs",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
		},
		"file_format": schema.StringAttribute{
			MarkdownDescription: "Log file format. Possible values: `json-list`, `json-object`, `json-line-delimited`, `csv`",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("json-list"),
			Validators: []validator.String{
				stringvalidator.OneOf("json-list", "json-object", "json-line-delimited", "csv"),
			},
		},
		"aws_s3": schema.SingleNestedAttribute{
			MarkdownDescription: "AWS S3 log destination",
			Optional:            true,
			Attributes:          AwsS3LogDestinationAttributes(),
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("compatible_s3"),
				),
			},
		},
		"compatible_s3": schema.SingleNestedAttribute{
			MarkdownDescription: "Compatible S3 log destination",
			Optional:            true,
			Attributes:          CompatibleS3LogDestinationAttributes(),
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("aws_s3"),
				),
			},
		},
	}
}

func AwsS3LogDestinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "AWS S3 bucket name",
			Required:            true,
		},
		"path": schema.StringAttribute{
			MarkdownDescription: "AWS S3 log destination path",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("/"),
		},
		"region": schema.StringAttribute{
			MarkdownDescription: "AWS S3 log destination region",
			Required:            true,
		},
		"credentials": schema.SingleNestedAttribute{
			MarkdownDescription: "AWS S3 log destination credentials",
			Optional:            true,
			WriteOnly:           true,
			Attributes:          AwsCredsAttributes(),
			Validators: []validator.Object{
				objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("credentials_version")),
			},
		},
		"credentials_version": schema.Int64Attribute{
			MarkdownDescription: "Increment this value to trigger a credentials update. " +
				"Credentials are only sent to the backend when this value changes. " +
				"After import, set this to any value alongside credentials to push them.",
			Optional: true,
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
	}
}

func AwsCredsAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"access_key": schema.SingleNestedAttribute{
			MarkdownDescription: "AWS access key credentials",
			Optional:            true,
			WriteOnly:           true,
			Attributes: map[string]schema.Attribute{
				"access_key": schema.StringAttribute{
					MarkdownDescription: "AWS access key",
					Required:            true,
					WriteOnly:           true,
				},
				"secret_key": schema.StringAttribute{
					MarkdownDescription: "AWS secret key",
					Required:            true,
					WriteOnly:           true,
				},
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("assume_role"),
				),
			},
		},
		"assume_role": schema.SingleNestedAttribute{
			MarkdownDescription: "AWS assume role credentials",
			Optional:            true,
			WriteOnly:           true,
			Attributes: map[string]schema.Attribute{
				"role_arn": schema.StringAttribute{
					MarkdownDescription: "AWS role ARN",
					Required:            true,
					WriteOnly:           true,
				},
				"external_id": schema.StringAttribute{
					MarkdownDescription: "AWS external ID",
					Required:            true,
					WriteOnly:           true,
				},
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("access_key"),
				),
			},
		},
	}
}

func CompatibleS3LogDestinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Compatible S3 bucket name",
			Required:            true,
		},
		"path": schema.StringAttribute{
			MarkdownDescription: "Compatible S3 log destination path",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("/"),
		},
		"region": schema.StringAttribute{
			MarkdownDescription: "Compatible S3 log destination region",
			Required:            true,
		},
		"domain": schema.StringAttribute{
			MarkdownDescription: "Compatible S3 log destination domain",
			Required:            true,
		},
		"credentials": schema.SingleNestedAttribute{
			MarkdownDescription: "Compatible S3 log destination credentials",
			Optional:            true,
			WriteOnly:           true,
			Attributes:          AwsCredsAttributes(),
			Validators: []validator.Object{
				objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("credentials_version")),
			},
		},
		"credentials_version": schema.Int64Attribute{
			MarkdownDescription: "Increment this value to trigger a credentials update. " +
				"Credentials are only sent to the backend when this value changes. " +
				"After import, set this to any value alongside credentials to push them.",
			Optional: true,
			Validators: []validator.Int64{
				int64validator.AtLeast(1),
			},
		},
	}
}

// redactedLogDestinationsForLog returns a printable representation of the outbound
// log-destinations array with the JSON-serialized credentials blob replaced so it
// is safe to hand to tflog. The original slice is never mutated.
func redactedLogDestinationsForLog(logDests []interface{}) string {
	return redactedCollectionForLog(logDests, func(copyMap map[string]interface{}) {
		redactMapKeys(copyMap, "credentials")
	})
}

func LogDestinationsToMap(ctx context.Context, logDestinations *[]LogDestinationModel, updateTransformCtx *ServiceTransformContext) ([]map[string]interface{}, error) {
	if logDestinations == nil {
		return nil, nil
	}

	newLogDestNamesToUUIDs := make(map[string]string)
	newDesiredOrder := []string{}

	var logDestArray []map[string]interface{}
	for i := range *logDestinations {
		logDest := &(*logDestinations)[i]
		logDestMap := logDest.ModelToMap()
		if logDestMap == nil {
			continue
		}

		name := logDest.Name.ValueString()
		uuid := logDest.Uuid.ValueString()
		if uuid == "" {
			uuid = GenerateUUID()
		}
		logDestMap["uuid"] = uuid
		newLogDestNamesToUUIDs[name] = uuid
		newDesiredOrder = append(newDesiredOrder, name)

		logDestArray = append(logDestArray, logDestMap)
	}

	// Atomically update the map and desired order
	updateTransformCtx.LogDestNamesToUUIDs = newLogDestNamesToUUIDs
	updateTransformCtx.DesiredLogDestOrder = newDesiredOrder

	return logDestArray, nil
}

// mergeLogDestCredentialsFromConfig copies WriteOnly credentials from a
// config-sourced model into a plan-sourced model, matching log destinations by
// name. Credentials are only injected when credentials_version changed vs state,
// so unchanged credentials are not re-sent to the backend on every update.
// On create, stateData is nil → credentials are always injected.
func mergeLogDestCredentialsFromConfig(planData, configData, stateData *ServiceResourceModel) {
	if planData.Config == nil || configData.Config == nil {
		return
	}
	if planData.Config.LogDestinations.IsNull() || planData.Config.LogDestinations.IsUnknown() ||
		configData.Config.LogDestinations.IsNull() || configData.Config.LogDestinations.IsUnknown() {
		return
	}

	planLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), planData.Config.LogDestinations)
	if err != nil {
		return
	}

	configLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), configData.Config.LogDestinations)
	if err != nil {
		return
	}

	// Build state lookup by name (nil-safe)
	stateByName := make(map[string]*LogDestinationModel)
	if stateData != nil && stateData.Config != nil && !stateData.Config.LogDestinations.IsNull() && !stateData.Config.LogDestinations.IsUnknown() {
		stateLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), stateData.Config.LogDestinations)
		if err == nil {
			for i := range stateLogDests {
				ld := &stateLogDests[i]
				stateByName[ld.Name.ValueString()] = ld
			}
		}
	}

	configByName := make(map[string]*LogDestinationModel, len(configLogDests))
	for i := range configLogDests {
		ld := &configLogDests[i]
		configByName[ld.Name.ValueString()] = ld
	}

	changed := false
	for i := range planLogDests {
		ld := &planLogDests[i]
		configLd, ok := configByName[ld.Name.ValueString()]
		if !ok {
			continue
		}
		stateLd := stateByName[ld.Name.ValueString()] // nil if new or post-import

		if ld.AwsS3 != nil && configLd.AwsS3 != nil {
			planVer := ld.AwsS3.CredentialsVersion
			var stateVer types.Int64
			if stateLd != nil && stateLd.AwsS3 != nil {
				stateVer = stateLd.AwsS3.CredentialsVersion
			}
			if shouldRotateLogDestinationCredentials(planVer, stateVer) {
				ld.AwsS3.Credentials = configLd.AwsS3.Credentials
				changed = true
			}
		}
		if ld.CompatibleS3 != nil && configLd.CompatibleS3 != nil {
			planVer := ld.CompatibleS3.CredentialsVersion
			var stateVer types.Int64
			if stateLd != nil && stateLd.CompatibleS3 != nil {
				stateVer = stateLd.CompatibleS3.CredentialsVersion
			}
			if shouldRotateLogDestinationCredentials(planVer, stateVer) {
				ld.CompatibleS3.Credentials = configLd.CompatibleS3.Credentials
				changed = true
			}
		}
	}

	if !changed {
		return
	}

	newList, err := ListObjectValueFrom(context.Background(), LogDestinationAttrTypes(), planLogDests)
	if err != nil {
		return
	}
	planData.Config.LogDestinations = newList
}

// shouldRotateLogDestinationCredentials returns true only when the plan sets a
// concrete credentials_version and it differs from state.
// Omitting credentials_version in config (null/unknown) is treated as "no
// explicit rotation request" and does not require (or inject) credentials.
func shouldRotateLogDestinationCredentials(planVer, stateVer types.Int64) bool {
	if planVer.IsNull() || planVer.IsUnknown() {
		return false
	}
	return !planVer.Equal(stateVer)
}

func hasConcreteLogDestinationCredentials(creds *AwsCredsModel) bool {
	if creds == nil {
		return false
	}

	if creds.AccessKey != nil {
		if creds.AccessKey.AccessKey.IsUnknown() && creds.AccessKey.SecretKey.IsUnknown() {
			return true
		}
		accessKeySet := !creds.AccessKey.AccessKey.IsNull() &&
			(creds.AccessKey.AccessKey.IsUnknown() || creds.AccessKey.AccessKey.ValueString() != "")
		secretKeySet := !creds.AccessKey.SecretKey.IsNull() &&
			(creds.AccessKey.SecretKey.IsUnknown() || creds.AccessKey.SecretKey.ValueString() != "")
		return accessKeySet && secretKeySet
	}

	if creds.AssumeRole != nil {
		if creds.AssumeRole.RoleArn.IsUnknown() && creds.AssumeRole.ExternalId.IsUnknown() {
			return true
		}
		roleArnSet := !creds.AssumeRole.RoleArn.IsNull() &&
			(creds.AssumeRole.RoleArn.IsUnknown() || creds.AssumeRole.RoleArn.ValueString() != "")
		externalIDSet := !creds.AssumeRole.ExternalId.IsNull() &&
			(creds.AssumeRole.ExternalId.IsUnknown() || creds.AssumeRole.ExternalId.ValueString() != "")
		return roleArnSet && externalIDSet
	}

	return false
}

func requireLogDestinationCredentials(name, reason string, creds *AwsCredsModel) error {
	if hasConcreteLogDestinationCredentials(creds) {
		return nil
	}
	return fmt.Errorf("log destination %q: %s requires credentials in config", name, reason)
}

func logDestinationNameOrUnnamed(name types.String) string {
	if name.IsNull() || name.IsUnknown() || name.ValueString() == "" {
		return "<unnamed>"
	}
	return name.ValueString()
}

func validateCreateLogDestinationCredentials(data *ServiceResourceModel) error {
	if data == nil || data.Config == nil || data.Config.LogDestinations.IsNull() || data.Config.LogDestinations.IsUnknown() {
		return nil
	}

	logDestinations, err := ListElementsAs[LogDestinationModel](context.Background(), data.Config.LogDestinations)
	if err != nil {
		return nil
	}

	for _, ld := range logDestinations {
		name := logDestinationNameOrUnnamed(ld.Name)

		if ld.AwsS3 != nil {
			if !ld.AwsS3.CredentialsVersion.IsNull() && !ld.AwsS3.CredentialsVersion.IsUnknown() {
				if err := requireLogDestinationCredentials(name, "setting aws_s3.credentials_version", ld.AwsS3.Credentials); err != nil {
					return err
				}
			}
		}

		if ld.CompatibleS3 != nil {
			if !ld.CompatibleS3.CredentialsVersion.IsNull() && !ld.CompatibleS3.CredentialsVersion.IsUnknown() {
				if err := requireLogDestinationCredentials(name, "setting compatible_s3.credentials_version", ld.CompatibleS3.Credentials); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateUpdateLogDestinationCredentials(planData, stateData *ServiceResourceModel) error {
	if planData == nil || planData.Config == nil || planData.Config.LogDestinations.IsNull() || planData.Config.LogDestinations.IsUnknown() {
		return nil
	}

	planLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), planData.Config.LogDestinations)
	if err != nil {
		return nil
	}

	stateByName := make(map[string]*LogDestinationModel)
	if stateData != nil && stateData.Config != nil && !stateData.Config.LogDestinations.IsNull() && !stateData.Config.LogDestinations.IsUnknown() {
		stateLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), stateData.Config.LogDestinations)
		if err == nil {
			for i := range stateLogDests {
				ld := &stateLogDests[i]
				stateByName[ld.Name.ValueString()] = ld
			}
		}
	}

	for i := range planLogDests {
		planLD := &planLogDests[i]
		name := logDestinationNameOrUnnamed(planLD.Name)
		stateLD, exists := stateByName[planLD.Name.ValueString()]

		if planLD.AwsS3 != nil {
			if !exists || stateLD == nil || stateLD.AwsS3 == nil {
				if !planLD.AwsS3.CredentialsVersion.IsNull() && !planLD.AwsS3.CredentialsVersion.IsUnknown() {
					if err := requireLogDestinationCredentials(name, "adding aws_s3 with credentials_version", planLD.AwsS3.Credentials); err != nil {
						return err
					}
				}
			} else if shouldRotateLogDestinationCredentials(planLD.AwsS3.CredentialsVersion, stateLD.AwsS3.CredentialsVersion) {
				if err := requireLogDestinationCredentials(name, "bumping aws_s3.credentials_version", planLD.AwsS3.Credentials); err != nil {
					return err
				}
			}
		}

		if planLD.CompatibleS3 != nil {
			if !exists || stateLD == nil || stateLD.CompatibleS3 == nil {
				if !planLD.CompatibleS3.CredentialsVersion.IsNull() && !planLD.CompatibleS3.CredentialsVersion.IsUnknown() {
					if err := requireLogDestinationCredentials(name, "adding compatible_s3 with credentials_version", planLD.CompatibleS3.Credentials); err != nil {
						return err
					}
				}
			} else if shouldRotateLogDestinationCredentials(planLD.CompatibleS3.CredentialsVersion, stateLD.CompatibleS3.CredentialsVersion) {
				if err := requireLogDestinationCredentials(name, "bumping compatible_s3.credentials_version", planLD.CompatibleS3.Credentials); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func credentialsPayloadJSON(creds *AwsCredsModel) (string, bool) {
	if creds == nil {
		return "", false
	}

	if creds.AccessKey != nil {
		payload := map[string]string{
			"access_key": creds.AccessKey.AccessKey.ValueString(),
			"secret_key": creds.AccessKey.SecretKey.ValueString(),
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", false
		}
		return string(b), true
	}

	if creds.AssumeRole != nil {
		payload := map[string]string{
			"role_arn":    creds.AssumeRole.RoleArn.ValueString(),
			"external_id": creds.AssumeRole.ExternalId.ValueString(),
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", false
		}
		return string(b), true
	}

	return "", false
}

// ModelToMap converts LogDestinationModel to the service config API format.
// The API stores log destinations as flat objects with fields:
//
//	name, type, s3_bucket, s3_path, s3_region, s3_domain, anonymize_ip, log_file_format, credentials
//
// Credentials are included in the payload on create/update but never returned
// by the API (WriteOnly), so they are not stored in state.
func (l *LogDestinationModel) ModelToMap() map[string]interface{} {
	if l == nil {
		return nil
	}

	logDestMap := make(map[string]interface{})
	logDestMap["name"] = l.Name.ValueString()
	logDestMap["anonymize_ip"] = l.AnonymizeIp.ValueBool()
	logDestMap["log_file_format"] = l.FileFormat.ValueString()

	if l.AwsS3 != nil {
		logDestMap["type"] = "S3"
		logDestMap["s3_bucket"] = l.AwsS3.Name.ValueString()
		logDestMap["s3_path"] = l.AwsS3.Path.ValueString()
		logDestMap["s3_region"] = l.AwsS3.Region.ValueString()
		logDestMap["s3_domain"] = "" // not used for AWS S3; empty string avoids JSON null
		if payload, ok := credentialsPayloadJSON(l.AwsS3.Credentials); ok {
			logDestMap["credentials"] = payload
		}
	} else if l.CompatibleS3 != nil {
		logDestMap["type"] = "S3_COMPATIBLE"
		logDestMap["s3_bucket"] = l.CompatibleS3.Name.ValueString()
		logDestMap["s3_path"] = l.CompatibleS3.Path.ValueString()
		logDestMap["s3_region"] = l.CompatibleS3.Region.ValueString()
		logDestMap["s3_domain"] = l.CompatibleS3.Domain.ValueString()
		if payload, ok := credentialsPayloadJSON(l.CompatibleS3.Credentials); ok {
			logDestMap["credentials"] = payload
		}
	}

	return logDestMap
}

func LogDestinationsFromMap(ctx context.Context, logDestArray []interface{}, updateTransformCtx *ServiceTransformContext) (*[]LogDestinationModel, error) {
	if logDestArray == nil {
		return nil, nil
	}

	desiredLogDestOrder := &updateTransformCtx.DesiredLogDestOrder
	newLogDestNamesToUUIDs := make(map[string]string)

	// Reverse map: UUID -> name (from prior state)
	nameByUuid := make(map[string]string)
	for name, uuid := range updateTransformCtx.LogDestNamesToUUIDs {
		nameByUuid[uuid] = name
	}

	var logDestinations []LogDestinationModel
	for _, logDest := range logDestArray {
		logDestMap, ok := logDest.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid log destination format")
		}

		logDestModel, err := MapToModel(logDestMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert log destination: %w", err)
		}

		uuid, _ := logDestMap["uuid"].(string)
		apiName, _ := logDestMap["name"].(string)

		// Resolve name: prefer prior-state name (matched by UUID), fall back to API name
		if priorName, exists := nameByUuid[uuid]; exists {
			logDestModel.Name = types.StringValue(priorName)
			newLogDestNamesToUUIDs[priorName] = uuid
		} else {
			logDestModel.Name = types.StringValue(apiName)
			newLogDestNamesToUUIDs[apiName] = uuid
		}
		logDestModel.Uuid = types.StringValue(uuid)

		logDestinations = append(logDestinations, *logDestModel)
	}

	// Update transform context
	updateTransformCtx.LogDestNamesToUUIDs = newLogDestNamesToUUIDs

	// Re-order to match HCL order; append any new items (e.g. from import) at the end
	reordered := alignItems(logDestinations, *desiredLogDestOrder)

	newDesiredOrder := make([]string, 0, len(reordered))
	for _, d := range reordered {
		newDesiredOrder = append(newDesiredOrder, d.Name.ValueString())
	}
	*desiredLogDestOrder = newDesiredOrder

	return &reordered, nil
}

// MapToModel converts the flat service config API format back to LogDestinationModel.
// The API format is:
//
//	{ "uuid": "...", "name": "...", "type": "S3"|"S3_COMPATIBLE",
//	  "s3_bucket": "...", "s3_path": "...", "s3_region": "...", "s3_domain": "..." }
func MapToModel(logDestMap map[string]interface{}) (*LogDestinationModel, error) {
	l := &LogDestinationModel{}

	if name, ok := logDestMap["name"].(string); ok {
		l.Name = types.StringValue(name)
	}

	destType, _ := logDestMap["type"].(string)

	anonymizeIp, _ := logDestMap["anonymize_ip"].(bool)
	fileFormat, _ := logDestMap["log_file_format"].(string)
	s3Bucket, _ := logDestMap["s3_bucket"].(string)
	s3Path, _ := logDestMap["s3_path"].(string)
	s3Region, _ := logDestMap["s3_region"].(string)
	s3Domain, _ := logDestMap["s3_domain"].(string)

	switch destType {
	case "S3":
		l.AwsS3 = &EmbeddedAwsS3LogDestinationModel{
			Name:   types.StringValue(s3Bucket),
			Path:   types.StringValue(s3Path),
			Region: types.StringValue(s3Region),
			// Credentials is WriteOnly — not stored in state, not returned by API.
		}
	case "S3_COMPATIBLE":
		l.CompatibleS3 = &EmbeddedCompatibleS3LogDestinationModel{
			Name:   types.StringValue(s3Bucket),
			Path:   types.StringValue(s3Path),
			Region: types.StringValue(s3Region),
			Domain: types.StringValue(s3Domain),
			// Credentials is WriteOnly — not stored in state, not returned by API.
		}
	default:
		return nil, fmt.Errorf("unsupported log destination type: %q", destType)
	}

	l.AnonymizeIp = types.BoolValue(anonymizeIp)
	l.FileFormat = types.StringValue(fileFormat)

	return l, nil
}
