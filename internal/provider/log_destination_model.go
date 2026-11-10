package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
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
	Name                types.String                             `tfsdk:"name"`
	Uuid                types.String                             `tfsdk:"uuid"`
	Source              types.String                             `tfsdk:"source"`
	AnonymizeIp         types.Bool                               `tfsdk:"anonymize_ip"`
	FileFormat          types.String                             `tfsdk:"file_format"`
	AwsS3               *EmbeddedAwsS3LogDestinationModel        `tfsdk:"aws_s3"`
	CompatibleS3        *EmbeddedCompatibleS3LogDestinationModel `tfsdk:"compatible_s3"`
	Splunk              *EmbeddedSplunkLogDestinationModel       `tfsdk:"splunk"`
	SelectedFields      types.List                               `tfsdk:"selected_fields"`
	RequestHeaders      types.List                               `tfsdk:"request_headers"`
	ResponseHeaders     types.List                               `tfsdk:"response_headers"`
	TimestampFormat     types.String                             `tfsdk:"timestamp_format"`
	ObjectName          types.String                             `tfsdk:"object_name"`
	MaxSamplesPerObject types.Int64                              `tfsdk:"max_samples_per_object"`
	MaxGapSeconds       types.Int64                              `tfsdk:"max_gap_seconds"`
}

type EmbeddedSplunkLogDestinationModel struct {
	URL                types.String      `tfsdk:"url"`
	ChannelID          types.String      `tfsdk:"channel_id"`
	UseInsecure        types.Bool        `tfsdk:"use_insecure"`
	Credentials        *SplunkCredsModel `tfsdk:"credentials"`
	CredentialsVersion types.Int64       `tfsdk:"credentials_version"`
}

type SplunkCredsModel struct {
	Token types.String `tfsdk:"token"`
}

func EmbeddedSplunkAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"url":                 types.StringType,
		"channel_id":          types.StringType,
		"use_insecure":        types.BoolType,
		"credentials":         types.ObjectType{AttrTypes: map[string]attr.Type{"token": types.StringType}},
		"credentials_version": types.Int64Type,
	}
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
		"name":                   types.StringType,
		"uuid":                   types.StringType,
		"source":                 types.StringType,
		"anonymize_ip":           types.BoolType,
		"file_format":            types.StringType,
		"aws_s3":                 types.ObjectType{AttrTypes: EmbeddedAwsS3AttrTypes()},
		"compatible_s3":          types.ObjectType{AttrTypes: EmbeddedCompatibleS3AttrTypes()},
		"splunk":                 types.ObjectType{AttrTypes: EmbeddedSplunkAttrTypes()},
		"selected_fields":        types.ListType{ElemType: types.StringType},
		"request_headers":        types.ListType{ElemType: types.StringType},
		"response_headers":       types.ListType{ElemType: types.StringType},
		"timestamp_format":       types.StringType,
		"object_name":            types.StringType,
		"max_samples_per_object": types.Int64Type,
		"max_gap_seconds":        types.Int64Type,
	}
}

func LogDestinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			MarkdownDescription: "Log destination name",
			Required:            true,
		},
		"uuid": schema.StringAttribute{
			MarkdownDescription: "Log destination UUID (managed by system)",
			Computed:            true,
			// We Do NOT use UseStateForUnknown() here.
			// The NamedListPlanModifier resolves uuid from state
		},
		"source": schema.StringAttribute{
			MarkdownDescription: "Log source: `traffic` (default) or `security`. Changing it replaces only the destination (new UUID), not the service; credentials must be supplied again.",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("traffic"),
			Validators: []validator.String{
				stringvalidator.OneOf("traffic", "security"),
			},
		},
		"selected_fields": schema.ListAttribute{
			Optional:    true,
			Computed:    true,
			ElementType: types.StringType,
			Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
			MarkdownDescription: "Ordered log fields to deliver. Omit for backend defaults based on source and file_format. If configured, the list must be nonempty and include timestamp, service_id and service_uid.\n" +
				"Choose fields for the destination's single source:\n" +
				"  - Traffic: `" + strings.Join(commonLogFields, "`, `") + "`, `" + strings.Join(trafficLogFields, "`, `") + "`.\n" +
				"  - Security: `" + strings.Join(commonLogFields, "`, `") + "`, `" + strings.Join(securityLogFields, "`, `") + "`.",
		},
		"request_headers":  logDestinationListAttribute("Request header names to collect when request_headers is selected (traffic logs only)."),
		"response_headers": logDestinationListAttribute("Response header names to collect when response_headers is selected (traffic logs only)."),
		"timestamp_format": schema.StringAttribute{
			MarkdownDescription: "Timestamp format: iso8601 (default) or unix.",
			Optional:            true, Computed: true,
			Default:    stringdefault.StaticString("iso8601"),
			Validators: []validator.String{stringvalidator.OneOf("iso8601", "unix")},
		},
		"object_name": schema.StringAttribute{
			MarkdownDescription: "Optional log filename template, for S3 destinations. Must include {random} and end in .gz; omit for backend naming.",
			Optional:            true,
			Validators:          []validator.String{stringvalidator.LengthBetween(1, 1024)},
		},
		"max_samples_per_object": schema.Int64Attribute{
			MarkdownDescription: "Maximum samples per log object (10000-250000; default 100000).",
			Optional:            true, Computed: true,
			Default:    int64default.StaticInt64(100000),
			Validators: []validator.Int64{int64validator.Between(10000, 250000)},
		},
		"max_gap_seconds": schema.Int64Attribute{
			MarkdownDescription: "Maximum gap between log samples in seconds (60-600; default 60).",
			Optional:            true, Computed: true,
			Default:    int64default.StaticInt64(60),
			Validators: []validator.Int64{int64validator.Between(60, 600)},
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
					path.MatchRelative().AtParent().AtName("splunk"),
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
					path.MatchRelative().AtParent().AtName("splunk"),
				),
			},
		},
		"splunk": schema.SingleNestedAttribute{
			MarkdownDescription: "Splunk log destination",
			Optional:            true,
			Attributes:          SplunkLogDestinationAttributes(),
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("aws_s3"),
					path.MatchRelative().AtParent().AtName("compatible_s3"),
				),
			},
		},
	}
}

func logDestinationListAttribute(description string) schema.ListAttribute {
	return schema.ListAttribute{
		MarkdownDescription: description,
		Optional:            true, Computed: true,
		ElementType: types.StringType,
		Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
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

func SplunkLogDestinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"url": schema.StringAttribute{
			MarkdownDescription: "Format: https://<host>:<port>/services/collector/raw",
			Required:            true,
		},
		"channel_id": schema.StringAttribute{
			MarkdownDescription: "A unique channel uuid.",
			Required:            true,
		},
		"use_insecure": schema.BoolAttribute{
			MarkdownDescription: "May skip verification of the Splunk HEC endpoint's SSL certificate.",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
		},
		"credentials": schema.SingleNestedAttribute{
			MarkdownDescription: "Splunk HEC credentials. Required on create/replacement or when credentials_version changes. Never stored in state.",
			Optional:            true,
			WriteOnly:           true,
			Attributes:          SplunkCredsAttributes(),
			Validators: []validator.Object{
				objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("credentials_version")),
			},
		},
		"credentials_version": schema.Int64Attribute{
			MarkdownDescription: "Increment to push a new Splunk token. Omit on update to preserve existing credentials; after import, set a version alongside credentials to rotate.",
			Optional:            true,
			Validators:          []validator.Int64{int64validator.AtLeast(1)},
		},
	}
}

func SplunkCredsAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"token": schema.StringAttribute{
			MarkdownDescription: "Splunk HEC token",
			Required:            true,
			WriteOnly:           true,
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
		logDestMap, err := logDest.ModelToMap()
		if err != nil {
			return nil, fmt.Errorf("log destination %q: %w", logDest.Name.ValueString(), err)
		}
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
// name. Credentials use credentials_version, except on create/replacement,
// and are never retained in state.
func mergeLogDestCredentialsFromConfig(planData, configData, stateData *ServiceResourceModel) error {
	if planData.Config == nil || configData.Config == nil {
		return nil
	}
	if planData.Config.LogDestinations.IsNull() || planData.Config.LogDestinations.IsUnknown() ||
		configData.Config.LogDestinations.IsNull() || configData.Config.LogDestinations.IsUnknown() {
		return nil
	}

	planLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), planData.Config.LogDestinations)
	if err != nil {
		return fmt.Errorf("failed to decode planned log destinations: %w", err)
	}

	configLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), configData.Config.LogDestinations)
	if err != nil {
		return fmt.Errorf("failed to decode configured log destinations: %w", err)
	}

	// Build state lookup by name (nil-safe)
	stateByName := make(map[string]*LogDestinationModel)
	if stateData != nil && stateData.Config != nil && !stateData.Config.LogDestinations.IsNull() && !stateData.Config.LogDestinations.IsUnknown() {
		stateLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), stateData.Config.LogDestinations)
		if err != nil {
			return fmt.Errorf("failed to decode prior log destinations: %w", err)
		}
		for i := range stateLogDests {
			ld := &stateLogDests[i]
			stateByName[ld.Name.ValueString()] = ld
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
		if stateLd != nil && !ld.Source.IsUnknown() {
			if logDestinationSourceChanged(ld, stateLd) {
				ld.Uuid = types.StringUnknown()
				changed = true
			} else if ld.Uuid.IsUnknown() {
				// A dynamic source can resolve to its prior value at apply.
				ld.Uuid = stateLd.Uuid
				changed = true
			}
		}

		if ld.AwsS3 != nil && configLd.AwsS3 != nil {
			planVer := ld.AwsS3.CredentialsVersion
			var stateVer types.Int64
			if stateLd != nil && stateLd.AwsS3 != nil {
				stateVer = stateLd.AwsS3.CredentialsVersion
			}
			if stateLd == nil || logDestinationTypeChanged(ld, stateLd) || logDestinationSourceChanged(ld, stateLd) || shouldRotateLogDestinationCredentials(planVer, stateVer) {
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
			if stateLd == nil || logDestinationTypeChanged(ld, stateLd) || logDestinationSourceChanged(ld, stateLd) || shouldRotateLogDestinationCredentials(planVer, stateVer) {
				ld.CompatibleS3.Credentials = configLd.CompatibleS3.Credentials
				changed = true
			}
		}
		if ld.Splunk != nil && configLd.Splunk != nil {
			var stateVer types.Int64
			if stateLd != nil && stateLd.Splunk != nil {
				stateVer = stateLd.Splunk.CredentialsVersion
			}
			if stateLd == nil || logDestinationTypeChanged(ld, stateLd) || logDestinationSourceChanged(ld, stateLd) ||
				shouldRotateLogDestinationCredentials(ld.Splunk.CredentialsVersion, stateVer) {
				ld.Splunk.Credentials = configLd.Splunk.Credentials
				changed = true
			}
		}
	}

	if !changed {
		return nil
	}

	newList, err := ListObjectValueFrom(context.Background(), LogDestinationAttrTypes(), planLogDests)
	if err != nil {
		return fmt.Errorf("failed to encode merged log destination credentials: %w", err)
	}
	planData.Config.LogDestinations = newList
	return nil
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

func credentialValueIsSet(value types.String, allowUnknown bool) bool {
	return !value.IsNull() && ((allowUnknown && value.IsUnknown()) ||
		(!value.IsUnknown() && value.ValueString() != ""))
}

func hasLogDestinationCredentials(creds *AwsCredsModel, allowUnknown bool) bool {
	if creds == nil {
		return false
	}

	if creds.AccessKey != nil {
		return credentialValueIsSet(creds.AccessKey.AccessKey, allowUnknown) &&
			credentialValueIsSet(creds.AccessKey.SecretKey, allowUnknown)
	}

	if creds.AssumeRole != nil {
		return credentialValueIsSet(creds.AssumeRole.RoleArn, allowUnknown) &&
			credentialValueIsSet(creds.AssumeRole.ExternalId, allowUnknown)
	}

	return false
}

func requireLogDestinationCredentials(name, reason string, creds *AwsCredsModel, allowUnknown bool) error {
	if hasLogDestinationCredentials(creds, allowUnknown) {
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

func validateCreateLogDestinationCredentials(data *ServiceResourceModel, allowUnknown bool) error {
	if data == nil || data.Config == nil || data.Config.LogDestinations.IsNull() || data.Config.LogDestinations.IsUnknown() {
		return nil
	}

	logDestinations, err := ListElementsAs[LogDestinationModel](context.Background(), data.Config.LogDestinations)
	if err != nil {
		return fmt.Errorf("failed to decode log destinations: %w", err)
	}

	for _, ld := range logDestinations {
		name := logDestinationNameOrUnnamed(ld.Name)

		if ld.AwsS3 != nil {
			if err := requireLogDestinationCredentials(name, "creating aws_s3", ld.AwsS3.Credentials, allowUnknown); err != nil {
				return err
			}
		}

		if ld.CompatibleS3 != nil {
			if err := requireLogDestinationCredentials(name, "creating compatible_s3", ld.CompatibleS3.Credentials, allowUnknown); err != nil {
				return err
			}
		}
		if ld.Splunk != nil {
			if err := requireSplunkCredentials(name, ld.Splunk.Credentials, allowUnknown); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateUpdateLogDestinationCredentials(planData, stateData *ServiceResourceModel, allowUnknown bool) error {
	if planData == nil || planData.Config == nil || planData.Config.LogDestinations.IsNull() || planData.Config.LogDestinations.IsUnknown() {
		return nil
	}

	planLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), planData.Config.LogDestinations)
	if err != nil {
		return fmt.Errorf("failed to decode log destinations: %w", err)
	}

	stateByName := make(map[string]*LogDestinationModel)
	if stateData != nil && stateData.Config != nil && !stateData.Config.LogDestinations.IsNull() && !stateData.Config.LogDestinations.IsUnknown() {
		stateLogDests, err := ListElementsAs[LogDestinationModel](context.Background(), stateData.Config.LogDestinations)
		if err != nil {
			return fmt.Errorf("failed to decode prior log destinations: %w", err)
		}
		for i := range stateLogDests {
			ld := &stateLogDests[i]
			stateByName[ld.Name.ValueString()] = ld
		}
	}

	for i := range planLogDests {
		planLD := &planLogDests[i]
		name := logDestinationNameOrUnnamed(planLD.Name)
		stateLD, exists := stateByName[planLD.Name.ValueString()]
		if logDestinationSourceChanged(planLD, stateLD) || logDestinationTypeChanged(planLD, stateLD) {
			reason := "source replacement"
			if logDestinationTypeChanged(planLD, stateLD) {
				reason = "destination type change"
			}
			if planLD.AwsS3 != nil {
				if err := requireLogDestinationCredentials(name, reason, planLD.AwsS3.Credentials, allowUnknown); err != nil {
					return err
				}
			}
			if planLD.CompatibleS3 != nil {
				if err := requireLogDestinationCredentials(name, reason, planLD.CompatibleS3.Credentials, allowUnknown); err != nil {
					return err
				}
			}
			if planLD.Splunk != nil {
				if err := requireSplunkCredentials(name, planLD.Splunk.Credentials, allowUnknown); err != nil {
					return err
				}
			}
			continue
		}

		if planLD.AwsS3 != nil {
			if !exists || stateLD == nil || stateLD.AwsS3 == nil {
				if !planLD.AwsS3.CredentialsVersion.IsNull() && !planLD.AwsS3.CredentialsVersion.IsUnknown() {
					if err := requireLogDestinationCredentials(name, "adding aws_s3 with credentials_version", planLD.AwsS3.Credentials, allowUnknown); err != nil {
						return err
					}
				}
			} else if shouldRotateLogDestinationCredentials(planLD.AwsS3.CredentialsVersion, stateLD.AwsS3.CredentialsVersion) {
				if err := requireLogDestinationCredentials(name, "bumping aws_s3.credentials_version", planLD.AwsS3.Credentials, allowUnknown); err != nil {
					return err
				}
			}
		}

		if planLD.CompatibleS3 != nil {
			if !exists || stateLD == nil || stateLD.CompatibleS3 == nil {
				if !planLD.CompatibleS3.CredentialsVersion.IsNull() && !planLD.CompatibleS3.CredentialsVersion.IsUnknown() {
					if err := requireLogDestinationCredentials(name, "adding compatible_s3 with credentials_version", planLD.CompatibleS3.Credentials, allowUnknown); err != nil {
						return err
					}
				}
			} else if shouldRotateLogDestinationCredentials(planLD.CompatibleS3.CredentialsVersion, stateLD.CompatibleS3.CredentialsVersion) {
				if err := requireLogDestinationCredentials(name, "bumping compatible_s3.credentials_version", planLD.CompatibleS3.Credentials, allowUnknown); err != nil {
					return err
				}
			}
		}
		if planLD.Splunk != nil {
			if !exists || stateLD == nil || stateLD.Splunk == nil {
				if err := requireSplunkCredentials(name, planLD.Splunk.Credentials, allowUnknown); err != nil {
					return err
				}
			} else if shouldRotateLogDestinationCredentials(planLD.Splunk.CredentialsVersion, stateLD.Splunk.CredentialsVersion) {
				if err := requireSplunkCredentials(name, planLD.Splunk.Credentials, allowUnknown); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func logDestinationSource(source types.String) string {
	if source.IsNull() || source.ValueString() == "" {
		return "traffic"
	}
	return strings.ToLower(source.ValueString())
}

func logDestinationSourceChanged(plan, state *LogDestinationModel) bool {
	return plan != nil && state != nil && !plan.Source.IsUnknown() &&
		logDestinationSource(plan.Source) != logDestinationSource(state.Source)
}

func logDestinationTypeChanged(plan, state *LogDestinationModel) bool {
	return plan != nil && state != nil &&
		((plan.AwsS3 != nil) != (state.AwsS3 != nil) ||
			(plan.CompatibleS3 != nil) != (state.CompatibleS3 != nil) ||
			(plan.Splunk != nil) != (state.Splunk != nil))
}

func requireSplunkCredentials(name string, credentials *SplunkCredsModel, allowUnknown bool) error {
	if credentials != nil && credentialValueIsSet(credentials.Token, allowUnknown) {
		return nil
	}
	return fmt.Errorf("log destination %q: creating/replacing splunk or rotating credentials requires credentials.token in config", name)
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
func (l *LogDestinationModel) ModelToMap() (map[string]interface{}, error) {
	if l == nil {
		return nil, nil
	}
	for key, value := range map[string]attr.Value{
		"source": l.Source, "object_name": l.ObjectName, "timestamp_format": l.TimestampFormat,
		"max_samples_per_object": l.MaxSamplesPerObject, "max_gap_seconds": l.MaxGapSeconds,
	} {
		if value.IsUnknown() {
			return nil, fmt.Errorf("%s must be known at apply", key)
		}
	}

	logDestMap := make(map[string]interface{})
	logDestMap["name"] = l.Name.ValueString()
	logDestMap["anonymize_ip"] = l.AnonymizeIp.ValueBool()
	logDestMap["log_file_format"] = l.FileFormat.ValueString()
	source := logDestinationSource(l.Source)
	if source != "traffic" && source != "security" {
		return nil, fmt.Errorf("unsupported log source %q", source)
	}
	logDestMap["source"] = strings.ToUpper(source)
	for key, value := range map[string]types.List{
		"selected_fields": l.SelectedFields, "request_headers": l.RequestHeaders, "response_headers": l.ResponseHeaders,
	} {
		// Omitted computed fields are unknown until the API returns its defaults.
		if key == "selected_fields" && (value.IsNull() || value.IsUnknown()) {
			continue
		}
		values := []string{}
		if !value.IsNull() {
			if diags := value.ElementsAs(context.Background(), &values, false); diags.HasError() {
				return nil, fmt.Errorf("invalid %s: %v", key, diags)
			}
		}
		logDestMap[key] = values
	}
	if !l.ObjectName.IsNull() {
		logDestMap["object_name"] = l.ObjectName.ValueString()
	}
	logDestMap["timestamp_format"] = "iso8601"
	if !l.TimestampFormat.IsNull() {
		logDestMap["timestamp_format"] = l.TimestampFormat.ValueString()
	}
	logDestMap["max_samples_per_object"] = int64(100000)
	if !l.MaxSamplesPerObject.IsNull() {
		logDestMap["max_samples_per_object"] = l.MaxSamplesPerObject.ValueInt64()
	}
	logDestMap["max_gap_seconds"] = int64(60)
	if !l.MaxGapSeconds.IsNull() {
		logDestMap["max_gap_seconds"] = l.MaxGapSeconds.ValueInt64()
	}

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
	} else if l.Splunk != nil {
		logDestMap["type"] = "SPLUNK"
		logDestMap["splunk"] = map[string]interface{}{
			"url":          l.Splunk.URL.ValueString(),
			"channel_id":   l.Splunk.ChannelID.ValueString(),
			"use_insecure": l.Splunk.UseInsecure.ValueBool(),
		}
		// The full service-config schema requires these strings even for Splunk.
		logDestMap["s3_path"] = "/"
		logDestMap["s3_region"] = "eu-central-1"
		if l.Splunk.Credentials != nil {
			if l.Splunk.Credentials.Token.IsNull() || l.Splunk.Credentials.Token.IsUnknown() || l.Splunk.Credentials.Token.ValueString() == "" {
				return nil, fmt.Errorf("Splunk credentials.token must be nonempty and known at apply")
			}
			payload, err := json.Marshal(map[string]string{"token": l.Splunk.Credentials.Token.ValueString()})
			if err != nil {
				return nil, fmt.Errorf("failed to encode Splunk credentials: %w", err)
			}
			logDestMap["credentials"] = string(payload)
		}
	}

	return logDestMap, nil
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
	source, _ := logDestMap["source"].(string)
	l.Source = types.StringValue(logDestinationSource(types.StringValue(source)))
	if l.Source.ValueString() != "traffic" && l.Source.ValueString() != "security" {
		return nil, fmt.Errorf("unsupported log source %q", source)
	}
	l.ObjectName = types.StringNull()
	if value, ok := logDestMap["object_name"].(string); ok {
		l.ObjectName = types.StringValue(value)
	}
	l.TimestampFormat = types.StringValue("iso8601")
	if value, ok := logDestMap["timestamp_format"].(string); ok {
		l.TimestampFormat = types.StringValue(value)
	}
	l.MaxSamplesPerObject = types.Int64Value(100000)
	l.MaxGapSeconds = types.Int64Value(60)
	for key, target := range map[string]*types.Int64{
		"max_samples_per_object": &l.MaxSamplesPerObject, "max_gap_seconds": &l.MaxGapSeconds,
	} {
		if raw, exists := logDestMap[key]; exists {
			value, ok := toFloat64(raw)
			if !ok {
				return nil, fmt.Errorf("invalid %s: %T", key, raw)
			}
			*target = types.Int64Value(int64(value))
		}
	}
	for key, target := range map[string]*types.List{
		"selected_fields": &l.SelectedFields, "request_headers": &l.RequestHeaders, "response_headers": &l.ResponseHeaders,
	} {
		raw := logDestMap[key]
		if raw == nil {
			raw = []string{}
		}
		value, diags := types.ListValueFrom(context.Background(), types.StringType, raw)
		if diags.HasError() {
			return nil, fmt.Errorf("invalid %s: %v", key, diags)
		}
		*target = value
	}

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
	case "SPLUNK":
		splunk, ok := logDestMap["splunk"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("SPLUNK destination is missing splunk configuration")
		}
		url, urlOK := splunk["url"].(string)
		channel, channelOK := splunk["channel_id"].(string)
		if !urlOK || !channelOK {
			return nil, fmt.Errorf("SPLUNK destination requires url and channel_id")
		}
		insecure, _ := splunk["use_insecure"].(bool)
		l.Splunk = &EmbeddedSplunkLogDestinationModel{
			URL: types.StringValue(url), ChannelID: types.StringValue(channel),
			UseInsecure: types.BoolValue(insecure),
		}
	default:
		return nil, fmt.Errorf("unsupported log destination type: %q", destType)
	}

	l.AnonymizeIp = types.BoolValue(anonymizeIp)
	l.FileFormat = types.StringValue(fileFormat)

	return l, nil
}

var commonLogFields = strings.Fields(`
timestamp service_id service_uid client_ip client_asn client_asn_name
geo_location geo_location_city domain path protocol range http_version
method query_params cookie referer x_forwarded_for user_agent
`)

var trafficLogFields = strings.Fields(`
unified_logs_behavior_id geo_location_subdivision status_code status_phrase
content_type content_encoding response_bytes provider detailed_result_type
server_ip is_cached midgress_bytes origin_bytes time_taken turnaround_time
request_headers response_headers
`)

var securityLogFields = strings.Fields(`
action mitigation_source custom_rule rate_limit_rule found_indicators severity_level
incident_details match_reason matched_location matched_parameter matched_sample
tls_version ja3 ja4 user_reputation
`)

var logHeaderNamePattern = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~a-zA-Z0-9]+$")
var logObjectNamePattern = regexp.MustCompile(`^[A-Za-z0-9/_\-.:%{}+^#]+$`)

func ValidateLogDestination(ctx context.Context, destination LogDestinationModel) []string {
	var errs []string
	if !destination.ObjectName.IsNull() && !destination.ObjectName.IsUnknown() {
		name := destination.ObjectName.ValueString()
		if !logObjectNamePattern.MatchString(name) || !strings.Contains(name, "{random}") || !strings.HasSuffix(name, ".gz") {
			errs = append(errs, "object_name must use supported template characters, include {random}, and end in .gz")
		}
	}
	headerCounts := make(map[string]int)
	totalHeaders := 0
	for key, value := range map[string]types.List{
		"request_headers": destination.RequestHeaders, "response_headers": destination.ResponseHeaders,
	} {
		if value.IsUnknown() {
			continue
		}
		headers, err := knownLogStrings(ctx, value)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
			continue
		}
		headerCounts[key] = len(headers)
		totalHeaders += len(headers)
		for _, header := range headers {
			if !header.IsUnknown() && (header.IsNull() || len(header.ValueString()) > 128 || !logHeaderNamePattern.MatchString(header.ValueString())) {
				errs = append(errs, fmt.Sprintf("%s contains an invalid header name", key))
			}
		}
	}
	if totalHeaders > 20 {
		errs = append(errs, "request_headers and response_headers must contain at most 20 names in total")
	}
	if destination.Source.IsUnknown() || destination.SelectedFields.IsUnknown() {
		return errs
	}
	fields, err := knownLogStrings(ctx, destination.SelectedFields)
	if err != nil {
		return append(errs, err.Error())
	}
	// Empty fields ask the backend to choose format/source-specific defaults.
	if len(fields) == 0 {
		return errs
	}
	allowed := make(map[string]bool)
	for _, field := range commonLogFields {
		allowed[field] = true
	}
	sourceFields := trafficLogFields
	if logDestinationSource(destination.Source) == "security" {
		sourceFields = securityLogFields
	}
	for _, field := range sourceFields {
		allowed[field] = true
	}
	selected := make(map[string]bool)
	for _, field := range fields {
		if field.IsUnknown() {
			return errs
		}
		name := field.ValueString()
		selected[name] = true
		if field.IsNull() || !allowed[name] {
			errs = append(errs, fmt.Sprintf("selected_fields contains unsupported %s field %q", logDestinationSource(destination.Source), name))
		}
	}
	for _, required := range []string{"timestamp", "service_id", "service_uid"} {
		if !selected[required] {
			errs = append(errs, fmt.Sprintf("selected_fields must include %s", required))
		}
	}
	for key, count := range headerCounts {
		if !selected[key] && count > 0 {
			errs = append(errs, fmt.Sprintf("%s must be empty unless selected_fields includes %s", key, key))
		}
		if selected[key] && count == 0 {
			errs = append(errs, fmt.Sprintf("%s must be nonempty when selected", key))
		}
	}
	return errs
}

func knownLogStrings(ctx context.Context, value types.List) ([]types.String, error) {
	if value.IsNull() {
		return nil, nil
	}
	var values []types.String
	if diags := value.ElementsAs(ctx, &values, false); diags.HasError() {
		return nil, fmt.Errorf("invalid log string list: %v", diags)
	}
	return values, nil
}

func validateLogDestinationReference(name types.String, source string, destinations map[string]LogDestinationModel, unknownNames bool) string {
	if name.IsNull() || name.IsUnknown() || name.ValueString() == "" {
		return ""
	}
	destination, exists := destinations[name.ValueString()]
	if !exists {
		if unknownNames {
			return ""
		}
		return fmt.Sprintf("log destination %q is not defined in config.log_destinations", name.ValueString())
	}
	if !destination.Source.IsUnknown() && logDestinationSource(destination.Source) != source {
		return fmt.Sprintf("log destination %q must use source %q", name.ValueString(), source)
	}
	return ""
}
