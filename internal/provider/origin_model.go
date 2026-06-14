package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// shieldProviderNames is the canonical list of valid TF-side provider names for origin shield.
// Must match the providers supported by the account_provider resource.
var shieldProviderNames = []string{
	"fastly",
	"cloudflare",
	"cloudfront",
	"gcp_cloud_cdn",
	"gcp_media_cdn",
	"akamai",
}

type OriginShieldLocationModel struct {
	Country     types.String `tfsdk:"country"`
	Subdivision types.String `tfsdk:"subdivision"`
}

type OriginShieldModel struct {
	Location  *OriginShieldLocationModel `tfsdk:"location"`
	Providers types.Set                  `tfsdk:"providers"`
}

// For use in ServiceConfigModel - matches service config API
type OriginModel struct {
	Uuid        types.String       `tfsdk:"uuid"`
	Name        types.String       `tfsdk:"name"`
	Path        types.String       `tfsdk:"path"`
	VerifySSL   types.Bool         `tfsdk:"verify_ssl"`
	TimeoutMs   types.Int64        `tfsdk:"timeout_ms"`
	SNIHostname types.String       `tfsdk:"sni_hostname"`
	Shield      *OriginShieldModel `tfsdk:"shield"`

	// Exactly ONE of these
	CustomOrigin *CustomOriginModel `tfsdk:"custom_origin"`
	S3Origin     *S3OriginModel     `tfsdk:"s3_origin"`
}

// Implement "Nameable" interface in utils.go
func (o OriginModel) GetName() string {
	return o.Name.ValueString()
}

type CustomOriginModel struct {
	Host            types.String `tfsdk:"host"`
	Protocol        types.String `tfsdk:"protocol"`
	CustomHttpPort  types.Int64  `tfsdk:"custom_http_port"`  // Optional
	CustomHttpsPort types.Int64  `tfsdk:"custom_https_port"` // Optional
}

type S3OriginModel struct {
	Host               types.String `tfsdk:"host"`
	IsStaticWebsite    types.Bool   `tfsdk:"is_static_website"`
	IsPrivate          types.Bool   `tfsdk:"is_private"`
	S3AwsRegion        types.String `tfsdk:"s3_aws_region"`       // If private
	S3BucketName       types.String `tfsdk:"s3_bucket_name"`      // If private
	S3AwsKey           types.String `tfsdk:"s3_aws_key"`          // WriteOnly — never stored in state, not returned by API
	S3AwsSecret        types.String `tfsdk:"s3_aws_secret"`       // WriteOnly — never stored in state, not returned by API
	CredentialsVersion types.Int64  `tfsdk:"credentials_version"` // TF-only counter; increment to push new credentials
}

// originBaseAttributes returns the schema attributes shared by both OriginModel
// and OriginSetOriginModel — everything except "name" and "shield".
func originBaseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"uuid": schema.StringAttribute{
			MarkdownDescription: "Origin UUID (managed by system)",
			Computed:            true,
			// We Do NOT use UseStateForUnknown() here.
			// The NamedListPlanModifier resolves uuid from state
		},
		"path": schema.StringAttribute{
			MarkdownDescription: "Path prefix for origin requests",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("/"),
		},
		"verify_ssl": schema.BoolAttribute{
			MarkdownDescription: "Verify SSL certificate when connecting to origin",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(true),
		},
		"timeout_ms": schema.Int64Attribute{
			MarkdownDescription: "Timeout in milliseconds for origin requests",
			Optional:            true,
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"sni_hostname": schema.StringAttribute{
			MarkdownDescription: "SNI hostname for TLS connection",
			Optional:            true,
		},
		"custom_origin": schema.SingleNestedAttribute{
			MarkdownDescription: "Custom origin configuration (HTTP/HTTPS server)",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"host": schema.StringAttribute{
					MarkdownDescription: "Origin hostname or IP address",
					Required:            true,
				},
				"protocol": schema.StringAttribute{
					MarkdownDescription: "Protocol to use when connecting to origin. Valid values: `http`, `https`, `http_and_https`",
					Optional:            true,
					Computed:            true,
					Validators: []validator.String{
						stringvalidator.OneOf("http", "https", "http_and_https"),
					},
					Default: stringdefault.StaticString("https"),
				},
				"custom_http_port": schema.Int64Attribute{
					MarkdownDescription: "Custom HTTP port (defaults to 80)",
					Optional:            true,
					Validators: []validator.Int64{
						int64validator.Between(1, 65535),
					},
				},
				"custom_https_port": schema.Int64Attribute{
					MarkdownDescription: "Custom HTTPS port (defaults to 443)",
					Optional:            true,
					Validators: []validator.Int64{
						int64validator.Between(1, 65535),
					},
				},
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("s3_origin")),
			},
		},
		"s3_origin": schema.SingleNestedAttribute{
			MarkdownDescription: "S3 bucket origin configuration",
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"host": schema.StringAttribute{
					MarkdownDescription: "S3 bucket hostname",
					Required:            true,
				},
				"is_static_website": schema.BoolAttribute{
					MarkdownDescription: "Is this an S3 static website",
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(false),
				},
				"is_private": schema.BoolAttribute{
					MarkdownDescription: "Is this a private S3 bucket",
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(false),
				},
				"s3_aws_region": schema.StringAttribute{
					MarkdownDescription: "AWS region (required if is_private = true)",
					Optional:            true,
				},
				"s3_bucket_name": schema.StringAttribute{
					MarkdownDescription: "S3 bucket name (required if is_private = true)",
					Optional:            true,
				},
				"s3_aws_key": schema.StringAttribute{
					MarkdownDescription: "AWS access key ID (required when is_private = true; write-only, never stored in state)",
					Optional:            true,
					WriteOnly:           true,
					Sensitive:           true,
					Validators: []validator.String{
						stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("credentials_version")),
					},
				},
				"s3_aws_secret": schema.StringAttribute{
					MarkdownDescription: "AWS secret access key (required when is_private = true; write-only, never stored in state)",
					Optional:            true,
					WriteOnly:           true,
					Sensitive:           true,
					Validators: []validator.String{
						stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("credentials_version")),
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
			},
			Validators: []validator.Object{
				objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("custom_origin")),
			},
		},
	}
}

func shieldNestedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"location": schema.SingleNestedAttribute{
			MarkdownDescription: "Geographic location of the origin shield",
			Required:            true,
			Attributes: map[string]schema.Attribute{
				"country": schema.StringAttribute{
					MarkdownDescription: "Country code where the shield is located (e.g. \"US\")",
					Required:            true,
				},
				"subdivision": schema.StringAttribute{
					MarkdownDescription: "Subdivision/state code (required when country is \"US\", e.g. \"TX\")",
					Optional:            true,
				},
			},
		},
		"providers": schema.SetAttribute{
			MarkdownDescription: "Set of CDN provider names to enable origin shield for (e.g. [\"fastly\", \"cloudflare\"])",
			Required:            true,
			ElementType:         types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(
					stringvalidator.OneOf(shieldProviderNames...),
				),
			},
		},
	}
}

func OriginAttributes() map[string]schema.Attribute {
	attrs := originBaseAttributes()
	attrs["name"] = schema.StringAttribute{
		MarkdownDescription: "Origin mapping ID (used for referencing in domain mappings)",
		Required:            true,
	}
	attrs["shield"] = schema.SingleNestedAttribute{
		MarkdownDescription: "Origin shield configuration.\n" +
			"  - Collapses requests at a chosen location before hitting the origin",
		Optional:   true,
		Attributes: shieldNestedAttributes(),
	}
	return attrs
}

func GetOriginAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":         types.StringType,
		"uuid":         types.StringType,
		"path":         types.StringType,
		"verify_ssl":   types.BoolType,
		"timeout_ms":   types.Int64Type,
		"sni_hostname": types.StringType,
		"shield": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"location": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"country":     types.StringType,
						"subdivision": types.StringType,
					},
				},
				"providers": types.SetType{ElemType: types.StringType},
			},
		},
		"custom_origin": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"host":              types.StringType,
				"protocol":          types.StringType,
				"custom_http_port":  types.Int64Type,
				"custom_https_port": types.Int64Type,
			},
		},
		"s3_origin": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"host":                types.StringType,
				"is_static_website":   types.BoolType,
				"is_private":          types.BoolType,
				"s3_aws_region":       types.StringType,
				"s3_bucket_name":      types.StringType,
				"s3_aws_key":          types.StringType,
				"s3_aws_secret":       types.StringType,
				"credentials_version": types.Int64Type,
			},
		},
	}
}

// GetOriginSetOriginAttrTypes returns attr types for OriginSetOriginModel —
// base fields only, no "name" and no "shield".
func GetOriginSetOriginAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid":         types.StringType,
		"path":         types.StringType,
		"verify_ssl":   types.BoolType,
		"timeout_ms":   types.Int64Type,
		"sni_hostname": types.StringType,
		"custom_origin": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"host":              types.StringType,
				"protocol":          types.StringType,
				"custom_http_port":  types.Int64Type,
				"custom_https_port": types.Int64Type,
			},
		},
		"s3_origin": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"host":                types.StringType,
				"is_static_website":   types.BoolType,
				"is_private":          types.BoolType,
				"s3_aws_region":       types.StringType,
				"s3_bucket_name":      types.StringType,
				"s3_aws_key":          types.StringType,
				"s3_aws_secret":       types.StringType,
				"credentials_version": types.Int64Type,
			},
		},
	}
}

// OriginsToMap converts slice of OriginModel to API array format
func OriginsToMap(ctx context.Context, origins *[]OriginModel, updateTransformCtx *ServiceTransformContext) ([]interface{}, error) {
	if origins == nil {
		return []interface{}{}, nil
	}

	originNamesToUUIDs := &updateTransformCtx.OriginNamesToUUIDs
	desiredOriginOrder := &updateTransformCtx.DesiredOriginOrder

	originsArray := []interface{}{}
	newOriginNameToUUIDMap := make(map[string]string)
	newDesiredOrder := []string{}

	// Step 1: Convert origins to API format (and generate UUIDs if needed)
	for i := range *origins {
		originMap, err := (*origins)[i].ModelToMap()
		if err != nil {
			return nil, fmt.Errorf("failed to convert origin: %w", err)
		}
		originsArray = append(originsArray, originMap)

		// Update map when UUID is now set / preserved
		name := (*origins)[i].Name.ValueString()
		uuid := (*origins)[i].Uuid.ValueString()
		newOriginNameToUUIDMap[name] = uuid
		newDesiredOrder = append(newDesiredOrder, name)
	}

	// Atomically replace the entire map
	*originNamesToUUIDs = newOriginNameToUUIDMap
	*desiredOriginOrder = newDesiredOrder

	return originsArray, nil
}

// ModelToMap converts OriginModel to API map format
func (o *OriginModel) ModelToMap() (map[string]interface{}, error) {
	originMap := map[string]interface{}{}

	// Generate UUID if not present (for new origins)
	if o.Uuid.IsNull() || o.Uuid.IsUnknown() {
		o.Uuid = types.StringValue(GenerateUUID())
	}

	// Add UUID if exists
	if !o.Uuid.IsNull() && !o.Uuid.IsUnknown() {
		originMap["uuid"] = o.Uuid.ValueString()
	}

	// Add path (required)
	if !o.Path.IsNull() && !o.Path.IsUnknown() {
		originMap["path"] = o.Path.ValueString()
	}

	// Add verify_ssl
	if !o.VerifySSL.IsNull() && !o.VerifySSL.IsUnknown() {
		originMap["verify_ssl"] = o.VerifySSL.ValueBool()
	}

	// Add timeout_ms if set
	if !o.TimeoutMs.IsNull() && !o.TimeoutMs.IsUnknown() {
		originMap["timeout_ms"] = o.TimeoutMs.ValueInt64()
	}

	// Add sni_hostname if set
	if !o.SNIHostname.IsNull() && !o.SNIHostname.IsUnknown() {
		originMap["sni_hostname"] = o.SNIHostname.ValueString()
	}

	// Add shield if present
	if o.Shield != nil {
		shieldMap := map[string]interface{}{}
		if o.Shield.Location != nil {
			locationMap := map[string]interface{}{
				"country": o.Shield.Location.Country.ValueString(),
			}
			if !o.Shield.Location.Subdivision.IsNull() && !o.Shield.Location.Subdivision.IsUnknown() {
				locationMap["subdivision"] = o.Shield.Location.Subdivision.ValueString()
			}
			shieldMap["location"] = locationMap
		}
		providers := []string{}
		for _, elem := range o.Shield.Providers.Elements() {
			if s, ok := elem.(types.String); ok {
				providers = append(providers, shieldProviderNameToBackend(s.ValueString()))
			}
		}
		shieldMap["providers"] = providers
		originMap["shield"] = shieldMap
	}

	// Add custom_origin if present
	if o.CustomOrigin != nil {
		customOriginMap := map[string]interface{}{
			"host":     o.CustomOrigin.Host.ValueString(),
			"protocol": originProtocolToBackend(o.CustomOrigin.Protocol.ValueString()),
		}

		// Add custom ports if set
		if !o.CustomOrigin.CustomHttpPort.IsNull() && !o.CustomOrigin.CustomHttpPort.IsUnknown() {
			customOriginMap["custom_http_port"] = o.CustomOrigin.CustomHttpPort.ValueInt64()
		}
		if !o.CustomOrigin.CustomHttpsPort.IsNull() && !o.CustomOrigin.CustomHttpsPort.IsUnknown() {
			customOriginMap["custom_https_port"] = o.CustomOrigin.CustomHttpsPort.ValueInt64()
		}

		originMap["custom_origin"] = customOriginMap
	}

	// Add s3_origin if present
	if o.S3Origin != nil {
		s3OriginMap := map[string]interface{}{
			"host": o.S3Origin.Host.ValueString(),
		}

		if !o.S3Origin.IsStaticWebsite.IsNull() && !o.S3Origin.IsStaticWebsite.IsUnknown() {
			s3OriginMap["is_static_website"] = o.S3Origin.IsStaticWebsite.ValueBool()
		}

		if !o.S3Origin.IsPrivate.IsNull() && !o.S3Origin.IsPrivate.IsUnknown() {
			isPrivate := o.S3Origin.IsPrivate.ValueBool()
			s3OriginMap["is_private"] = isPrivate

			// Add region and bucket name if private
			if isPrivate {
				if !o.S3Origin.S3AwsRegion.IsNull() && !o.S3Origin.S3AwsRegion.IsUnknown() {
					s3OriginMap["s3_aws_region"] = o.S3Origin.S3AwsRegion.ValueString()
				}
				if !o.S3Origin.S3BucketName.IsNull() && !o.S3Origin.S3BucketName.IsUnknown() {
					s3OriginMap["s3_bucket_name"] = o.S3Origin.S3BucketName.ValueString()
				}
			}
		}

		// Include AWS credentials if provided (WriteOnly — sent to API, not stored in state)
		if !o.S3Origin.S3AwsKey.IsNull() && !o.S3Origin.S3AwsKey.IsUnknown() {
			s3OriginMap["s3_aws_key"] = o.S3Origin.S3AwsKey.ValueString()
		}
		if !o.S3Origin.S3AwsSecret.IsNull() && !o.S3Origin.S3AwsSecret.IsUnknown() {
			s3OriginMap["s3_aws_secret"] = o.S3Origin.S3AwsSecret.ValueString()
		}

		originMap["s3_origin"] = s3OriginMap
	}

	return originMap, nil
}

// OriginsFromMap converts API array to slice of OriginModel
func OriginsFromMap(ctx context.Context, originsArray []interface{}, updateTransformCtx *ServiceTransformContext) (*[]OriginModel, error) {
	origins := []OriginModel{}
	originNamesToUUIDs := &updateTransformCtx.OriginNamesToUUIDs
	DesiredOriginOrder := &updateTransformCtx.DesiredOriginOrder

	// Reverse the map: UUID -> name
	nameByUuid := make(map[string]string)
	for name, uuid := range *originNamesToUUIDs {
		nameByUuid[uuid] = name
	}

	// Build new map from API response
	newOriginNamesToUUIDs := make(map[string]string)

	for _, originData := range originsArray {
		originMap, ok := originData.(map[string]interface{})
		if !ok {
			continue
		}

		origin, err := originFromMap(ctx, originMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert origin: %w", err)
		}

		if origin.Uuid.IsNull() || origin.Uuid.IsUnknown() {
			return nil, fmt.Errorf("[OriginsFromMap] Response missing UUID for some origins")
		}

		uuid := origin.Uuid.ValueString()

		if userName, exists := nameByUuid[uuid]; exists {
			origin.Name = types.StringValue(userName)
			newOriginNamesToUUIDs[userName] = uuid

			tflog.Debug(ctx, fmt.Sprintf("Matched origin UUID %s to existing name '%s'", uuid, userName))
		} else {
			// fallback to UUID as name
			origin.Name = types.StringValue(uuid)
			newOriginNamesToUUIDs[uuid] = uuid

			tflog.Debug(ctx, fmt.Sprintf("Fallback to UUID as name for origin: %s", uuid))
		}

		origins = append(origins, *origin)
	}

	// Update the transform context with new state
	*originNamesToUUIDs = newOriginNamesToUUIDs

	// Rebuild the list - first plan items as came in, after them, add new items(may come from import)
	reorderedOrigins := alignItems(origins, *DesiredOriginOrder)

	// New desired order
	newDesiredOrder := make([]string, 0, len(reorderedOrigins))
	for _, origin := range reorderedOrigins {
		newDesiredOrder = append(newDesiredOrder, origin.Name.ValueString())
	}
	*DesiredOriginOrder = newDesiredOrder

	return &reorderedOrigins, nil
}

// OriginFromMap converts API map to OriginModel
func originFromMap(ctx context.Context, originMap map[string]interface{}) (*OriginModel, error) {
	origin := &OriginModel{}

	// Extract UUID
	if uuid, ok := originMap["uuid"].(string); ok {
		origin.Uuid = types.StringValue(uuid)
	}

	// Extract path
	if path, ok := originMap["path"].(string); ok {
		origin.Path = types.StringValue(path)
	}

	// Extract verify_ssl
	if verifySSL, ok := originMap["verify_ssl"].(bool); ok {
		origin.VerifySSL = types.BoolValue(verifySSL)
	}

	// Extract timeout_ms
	if timeoutMs, ok := originMap["timeout_ms"].(float64); ok {
		origin.TimeoutMs = types.Int64Value(int64(timeoutMs))
	}

	// Extract sni_hostname
	if sniHostname, ok := originMap["sni_hostname"].(string); ok {
		origin.SNIHostname = types.StringValue(sniHostname)
	}

	// Extract shield
	if shieldData, ok := originMap["shield"].(map[string]interface{}); ok {
		shield := &OriginShieldModel{}
		if locationData, ok := shieldData["location"].(map[string]interface{}); ok {
			location := &OriginShieldLocationModel{}
			if country, ok := locationData["country"].(string); ok {
				location.Country = types.StringValue(country)
			}
			if subdivision, ok := locationData["subdivision"].(string); ok {
				location.Subdivision = types.StringValue(subdivision)
			} else {
				location.Subdivision = types.StringNull()
			}
			shield.Location = location
		}
		if providersData, ok := shieldData["providers"].([]interface{}); ok {
			elems := []attr.Value{}
			for _, p := range providersData {
				if pStr, ok := p.(string); ok {
					elems = append(elems, types.StringValue(shieldProviderNameFromBackend(pStr)))
				}
			}
			shield.Providers = types.SetValueMust(types.StringType, elems)
		} else {
			shield.Providers = types.SetValueMust(types.StringType, []attr.Value{})
		}
		origin.Shield = shield
	}

	// Extract custom_origin
	if customOriginMap, ok := originMap["custom_origin"].(map[string]interface{}); ok {
		customOrigin := &CustomOriginModel{}

		if host, ok := customOriginMap["host"].(string); ok {
			customOrigin.Host = types.StringValue(host)
		}

		if protocol, ok := customOriginMap["protocol"].(string); ok {
			customOrigin.Protocol = types.StringValue(originProtocolFromBackend(protocol))
		}

		if httpPort, ok := customOriginMap["custom_http_port"].(float64); ok {
			customOrigin.CustomHttpPort = types.Int64Value(int64(httpPort))
		}

		if httpsPort, ok := customOriginMap["custom_https_port"].(float64); ok {
			customOrigin.CustomHttpsPort = types.Int64Value(int64(httpsPort))
		}

		origin.CustomOrigin = customOrigin
	}

	// Extract s3_origin
	if s3OriginMap, ok := originMap["s3_origin"].(map[string]interface{}); ok {
		s3Origin := &S3OriginModel{}

		if host, ok := s3OriginMap["host"].(string); ok {
			s3Origin.Host = types.StringValue(host)
		}

		if isStatic, ok := s3OriginMap["is_static_website"].(bool); ok {
			s3Origin.IsStaticWebsite = types.BoolValue(isStatic)
		}

		if isPrivate, ok := s3OriginMap["is_private"].(bool); ok {
			s3Origin.IsPrivate = types.BoolValue(isPrivate)
		}

		if region, ok := s3OriginMap["s3_aws_region"].(string); ok {
			s3Origin.S3AwsRegion = types.StringValue(region)
		}

		if bucketName, ok := s3OriginMap["s3_bucket_name"].(string); ok {
			s3Origin.S3BucketName = types.StringValue(bucketName)
		}

		// WriteOnly — not returned by the API. Must be explicitly null so
		// types.ListValueFrom doesn't embed the Go zero-value ("") in the list object.
		// The actual values are re-injected from req.Config by mergeS3OriginCredentialsFromConfig on Update.
		s3Origin.S3AwsKey = types.StringNull()
		s3Origin.S3AwsSecret = types.StringNull()

		origin.S3Origin = s3Origin
	}

	tflog.Debug(ctx, fmt.Sprintf("[originFromMap] Converted origin: %+v", origin))
	return origin, nil
}

// OriginsFromMap converts API array to slice of OriginModel
func OriginModelsFromMap(ctx context.Context, originsArray []interface{}) ([]OriginModel, error) {
	origins := []OriginModel{}

	for _, originData := range originsArray {
		originMap, ok := originData.(map[string]interface{})
		if !ok {
			continue
		}

		origin, err := originFromMap(ctx, originMap)
		if err != nil {
			return origins, fmt.Errorf("failed to convert origin: %w", err)
		}

		if origin.Uuid.IsNull() || origin.Uuid.IsUnknown() {
			return origins, fmt.Errorf("[OriginsFromMap] Response missing UUID for some origins")
		}
		origins = append(origins, *origin)
	}

	return origins, nil
}

// originProtocolToBackend converts TF-friendly protocol value to backend value.
func originProtocolToBackend(p string) string {
	switch p {
	case "http":
		return "HTTP"
	case "https":
		return "HTTPS"
	case "http_and_https":
		return "HTTP & HTTPS"
	default:
		return p
	}
}

// originProtocolFromBackend converts backend protocol value to TF-friendly value.
func originProtocolFromBackend(p string) string {
	switch p {
	case "HTTP":
		return "http"
	case "HTTPS":
		return "https"
	case "HTTP & HTTPS":
		return "http_and_https"
	default:
		return p
	}
}

// shieldProviderNameToBackend converts a lowercase TF provider name
// (e.g. "fastly") to the ProviderName enum value the backend expects
// (e.g. "Fastly") in the service config JSON.
func shieldProviderNameToBackend(name string) string {
	switch name {
	case "fastly":
		return "Fastly"
	case "cloudflare":
		return "Cloudflare"
	case "cloudfront":
		return "Cloudfront"
	case "gcp_cloud_cdn":
		return "GCPCloudCDN"
	case "gcp_media_cdn":
		return "GCPMediaCDN"
	case "edgio":
		return "Edgio"
	case "akamai":
		return "Akamai"
	default:
		return name
	}
}

// shieldProviderNameFromBackend converts a backend ProviderName enum value
// (e.g. "Fastly") back to the lowercase TF name (e.g. "fastly").
func shieldProviderNameFromBackend(name string) string {
	switch name {
	case "Fastly":
		return "fastly"
	case "Cloudflare":
		return "cloudflare"
	case "Cloudfront":
		return "cloudfront"
	case "GCPCloudCDN":
		return "gcp_cloud_cdn"
	case "GCPMediaCDN":
		return "gcp_media_cdn"
	case "Edgio":
		return "edgio"
	case "Akamai":
		return "akamai"
	default:
		return name
	}
}

// mergeS3OriginCredentialsFromConfig copies WriteOnly credentials (s3_aws_key /
// s3_aws_secret) from a config-sourced model into a plan-sourced model, matching
// origins by name. Credentials are only injected when credentials_version changed
// vs state, so unchanged credentials are not re-sent on every update.
// On create, stateData is nil → credentials are always injected.
func mergeS3OriginCredentialsFromConfig(planData, configData, stateData *ServiceResourceModel) {
	if planData.Config == nil || configData.Config == nil {
		return
	}
	if planData.Config.Origins.IsNull() || planData.Config.Origins.IsUnknown() {
		return
	}
	if configData.Config.Origins.IsNull() || configData.Config.Origins.IsUnknown() {
		return
	}

	var planOrigins []OriginModel
	if diags := planData.Config.Origins.ElementsAs(context.Background(), &planOrigins, false); diags.HasError() {
		return
	}
	var configOrigins []OriginModel
	if diags := configData.Config.Origins.ElementsAs(context.Background(), &configOrigins, false); diags.HasError() {
		return
	}

	// Build state lookup by name (nil-safe)
	stateByName := make(map[string]*OriginModel)
	if stateData != nil && stateData.Config != nil {
		var stateOrigins []OriginModel
		if diags := stateData.Config.Origins.ElementsAs(context.Background(), &stateOrigins, false); !diags.HasError() {
			for i := range stateOrigins {
				o := &stateOrigins[i]
				stateByName[o.Name.ValueString()] = o
			}
		}
	}

	configByName := make(map[string]*OriginModel, len(configOrigins))
	for i := range configOrigins {
		o := &configOrigins[i]
		configByName[o.Name.ValueString()] = o
	}

	changed := false
	for i := range planOrigins {
		o := &planOrigins[i]
		configOrigin, ok := configByName[o.Name.ValueString()]
		if !ok || o.S3Origin == nil || configOrigin.S3Origin == nil {
			continue
		}

		planVer := o.S3Origin.CredentialsVersion
		var stateVer types.Int64
		if stateOrigin, exists := stateByName[o.Name.ValueString()]; exists && stateOrigin.S3Origin != nil {
			stateVer = stateOrigin.S3Origin.CredentialsVersion
		}

		if !planVer.Equal(stateVer) {
			o.S3Origin.S3AwsKey = configOrigin.S3Origin.S3AwsKey
			o.S3Origin.S3AwsSecret = configOrigin.S3Origin.S3AwsSecret
			changed = true
		}
	}

	if !changed {
		return
	}

	// Repack the modified slice back into the types.List
	elemType := types.ObjectType{AttrTypes: GetOriginAttrTypes()}
	newList, diags := types.ListValueFrom(context.Background(), elemType, planOrigins)
	if diags.HasError() {
		return
	}
	planData.Config.Origins = newList
}
