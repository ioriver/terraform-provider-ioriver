package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}
var _ resource.ResourceWithValidateConfig = &ServiceResource{}

func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

type ServiceResourceId = string
type ServiceResource struct {
	client *ioriver.IORiverClient
}

const (
	CurrentTransformCtxPrivateKeyName = "current_transform_ctx"
)

type ServiceTransformContext struct {
	OriginNamesToUUIDs    map[string]string `json:"origin_names_to_uuids"`
	DesiredOriginOrder    []string          `json:"desired_origin_order"`
	LogDestNamesToUUIDs   map[string]string `json:"log_dest_names_to_uuids"`
	DesiredLogDestOrder   []string          `json:"desired_log_dest_order"`
	DesiredDomainOrder    []string          `json:"desired_domain_order"`
	DesiredOriginSetOrder []string          `json:"desired_origin_set_order"`
	// DesiredMappingOrder tracks the HCL order of mappings per domain (keyed by domain name).
	DesiredMappingOrder map[string][]string `json:"desired_mapping_order"`
	// SecurityConfigured is true when the user explicitly set the security block.
	SecurityConfigured bool `json:"security_configured"`
	// BehaviorRepresentation records how each named behavior was written by the user.
	BehaviorRepresentation map[string]string `json:"behavior_representation,omitempty"`
	// OriginsExplicitlySet / OriginSetsExplicitlySet: true when the user wrote the
	// block (even as an empty list). Used by ServiceConfigMapToModel to return []
	// instead of null when the API sends back nothing, preserving plan consistency.
	OriginsExplicitlySet    bool `json:"origins_explicitly_set"`
	OriginSetsExplicitlySet bool `json:"origin_sets_explicitly_set"`
}

type ServiceResourceModel struct {
	Id                 types.String             `tfsdk:"id"`
	Name               types.String             `tfsdk:"name"`
	Description        types.String             `tfsdk:"description"`
	Cname              types.String             `tfsdk:"cname"`
	Certificate        types.String             `tfsdk:"certificate"`
	Config             *ServiceConfigModel      `tfsdk:"config"`
	updateTransformCtx *ServiceTransformContext // No tfsdk tag - not in schema!
}

func (r *ServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Service resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Service name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Service description",
				Optional:            true,
			},
			"certificate": schema.StringAttribute{
				MarkdownDescription: "ID of the certificate to be used with the service",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cname": schema.StringAttribute{
				MarkdownDescription: "CNAME for the IO River service",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"config": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Service configuration",
				Attributes:          ConfigAttributes(),
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *ServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create Service resource
func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// This is used during this flow for storing adapting fields
	data.updateTransformCtx = &ServiceTransformContext{
		OriginNamesToUUIDs:  make(map[string]string),
		DesiredOriginOrder:  []string{},
		LogDestNamesToUUIDs: make(map[string]string),
		DesiredLogDestOrder: []string{},
	}

	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)

	if newData == nil {
		return
	}

	if model, ok := newData.(ServiceResourceModel); ok {
		tflog.Debug(ctx, fmt.Sprintf("[Create] service created with ID: %s config_present=%v",
			model.Id.ValueString(), model.Config != nil))
	}

	// Save transform context in private state
	cfgJson, err := json.Marshal(data.updateTransformCtx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal update transform context",
			fmt.Sprintf("Error: %s", err))
		return
	}

	resp.Private.SetKey(ctx, CurrentTransformCtxPrivateKeyName, cfgJson)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read Service resource
func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	// Get/set transform context in state
	existTransformCtxByteArray, diags := req.Private.GetKey(ctx, CurrentTransformCtxPrivateKeyName)
	resp.Diagnostics.Append(diags...)

	data.updateTransformCtx = &ServiceTransformContext{
		OriginNamesToUUIDs:  make(map[string]string),
		DesiredOriginOrder:  []string{},
		LogDestNamesToUUIDs: make(map[string]string),
		DesiredLogDestOrder: []string{},
	}
	if len(existTransformCtxByteArray) > 0 {
		// Unmarshal existing context
		if err := json.Unmarshal(existTransformCtxByteArray, data.updateTransformCtx); err != nil {
			resp.Diagnostics.AddError("Failed to unmarshal existing transform context",
				fmt.Sprintf("Error: %s", err))
			return
		}
	}

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Save transform context in private state
	cfgJson, err := json.Marshal(data.updateTransformCtx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal update transform context",
			fmt.Sprintf("Error: %s", err))
		return
	}

	resp.Private.SetKey(ctx, CurrentTransformCtxPrivateKeyName, cfgJson)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update Service resource
func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceResourceModel
	var stateData ServiceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	// Read current state to get Computed fields (e.g. config.uuid) that are not in the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// WriteOnly credentials are not stored in state so the plan cannot carry them
	// from prior state. Re-read them from the raw config and merge by name,
	// but only inject when credentials_version changed vs state.
	var configData ServiceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if !resp.Diagnostics.HasError() {
		mergeWriteOnlyCredentialsFromConfig(&data, &configData, &stateData)
	}

	// Get/set transform context in state
	existTransformCtxByteArray, diags := req.Private.GetKey(ctx, CurrentTransformCtxPrivateKeyName)
	resp.Diagnostics.Append(diags...)

	data.updateTransformCtx = &ServiceTransformContext{
		OriginNamesToUUIDs:  make(map[string]string),
		DesiredOriginOrder:  []string{},
		LogDestNamesToUUIDs: make(map[string]string),
		DesiredLogDestOrder: []string{},
	}
	if len(existTransformCtxByteArray) > 0 {
		// Unmarshal existing context
		if err := json.Unmarshal(existTransformCtxByteArray, data.updateTransformCtx); err != nil {
			resp.Diagnostics.AddError("Failed to unmarshal existing transform context",
				fmt.Sprintf("Error: %s", err))
			return
		}
	}

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	// Save transform context in private state
	cfgJson, err := json.Marshal(data.updateTransformCtx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to marshal update transform context",
			fmt.Sprintf("Error: %s", err))
		return
	}

	resp.Private.SetKey(ctx, CurrentTransformCtxPrivateKeyName, cfgJson)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete Service resource
func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import Service resource
func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mergeWriteOnlyCredentialsFromConfig injects WriteOnly credentials from the raw
// config into the plan model before the API call. WriteOnly fields (s3_aws_key,
// s3_aws_secret, log-destination credentials) are null in the plan on Update
// because Terraform does not carry them from prior state. req.Config is the only
// source that still holds the values the user typed in HCL.
// Credentials are only forwarded when credentials_version changed vs state.
func mergeWriteOnlyCredentialsFromConfig(planData, configData, stateData *ServiceResourceModel) {
	mergeLogDestCredentialsFromConfig(planData, configData, stateData)
	mergeS3OriginCredentialsFromConfig(planData, configData, stateData)
}

// ------- Implement base Resource API ---------

func (ServiceResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return CreateServiceWithConfig(client, newObj.(ServiceWithConfig))
}

func (ServiceResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	return GetServiceWithConfig(client, id.(ServiceResourceId))
}

func (ServiceResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return UpdateServiceWithConfig(client, obj.(ServiceWithConfig))
}

func (ServiceResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	return DeleteServiceWithConfig(client, id.(ServiceResourceId))
}

func (ServiceResource) getId(data interface{}) interface{} {
	d := data.(ServiceResourceModel)
	return d.Id.ValueString()
}

// Convert Service resource to Service API object
func (ServiceResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ServiceResourceModel)

	var configMap map[string]interface{}
	if d.Config != nil {
		var err error
		configMap, err = d.Config.ModelToMap(ctx, d.updateTransformCtx)
		if err != nil {
			return nil, fmt.Errorf("resourceToObj: %w", err)
		}
	} else {
		configMap = make(map[string]interface{})
	}
	// Set the service name in the config on create (required by backend).
	// On update, creation_name is carried from state via UseStateForUnknown and
	// already serialized by ModelToMap — do not override it.
	if name, ok := configMap["name"].(string); !ok || name == "" {
		configMap["name"] = d.Name.ValueString()
	}

	tflog.Debug(ctx, fmt.Sprintf("[resourceToObj] Update Transform Context: %+v", d.updateTransformCtx))

	if d.updateTransformCtx != nil && d.Config != nil {
		d.updateTransformCtx.SecurityConfigured = !d.Config.Security.IsNull() && !d.Config.Security.IsUnknown()
	}

	certificate := d.Certificate.ValueString()

	return ServiceWithConfig{
		Id:           d.Id.ValueString(),
		Name:         d.Name.ValueString(),
		Description:  d.Description.ValueString(),
		Certificates: []string{certificate},
		Config:       configMap,
	}, nil
}

// Convert Service API object to Service resource
func (ServiceResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	service := obj.(*ServiceWithConfig)
	d := data.(ServiceResourceModel)

	// Debug: Print what the API returned
	tflog.Debug(ctx, fmt.Sprintf("[objToResource] API Response - ID: %s, Name: %s", service.Id, service.Name))
	if service.Config == nil {
		tflog.Warn(ctx, "[objToResource] ⚠️  service.Config is NIL from API!")
	}

	// During import there is no prior state, so d.Config is nil.
	// Treat a nil Config (import path) as "everything configured".
	if d.Config == nil && d.updateTransformCtx != nil {
		d.updateTransformCtx.SecurityConfigured = true
	}

	configModel, err := ServiceConfigMapToModel(ctx, service.Config, d.updateTransformCtx, d.Config)
	if err != nil {
		return nil, fmt.Errorf("objToResource: %w", err)
	}

	tflog.Debug(ctx, fmt.Sprintf("[objToResource] Update Transform Context: %+v", d.updateTransformCtx))

	// Debug: Print what we converted
	if configModel == nil {
		tflog.Warn(ctx, "[objToResource] ⚠️  configModel is NIL after conversion!")
	} else {
		tflog.Debug(ctx, "[objToResource] ✓ configModel populated")
	}

	return ServiceResourceModel{
		Id:          types.StringValue(service.Id),
		Name:        types.StringValue(service.Name),
		Description: types.StringValue(service.Description),
		Certificate: types.StringValue(service.Certificates[0]),
		Cname:       types.StringValue(service.Cname),
		Config:      configModel,
	}, nil
}

// ValidateConfig runs cross-field validation that cannot be expressed with
// schema-level validators alone (e.g. field_key required for collection fields).
// It is called by the framework automatically on every plan and apply.
func (r *ServiceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ServiceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Config == nil {
		return
	}

	// --- Security (WAF custom rules + rate limit) ---
	var secPtr *SecurityModel
	if !data.Config.Security.IsNull() && !data.Config.Security.IsUnknown() {
		var sec SecurityModel
		if diags := data.Config.Security.As(ctx, &sec, basetypes.ObjectAsOptions{}); !diags.HasError() {
			secPtr = &sec
		}
	}
	for _, msg := range ValidateSecurityModel(ctx, secPtr) {
		resp.Diagnostics.AddAttributeError(
			path.Root("config").AtName("security"),
			"Invalid security configuration",
			msg,
		)
	}

	// --- Specific behaviors (condition field_key constraints) ---
	if !data.Config.Behaviors.IsNull() && !data.Config.Behaviors.IsUnknown() {
		var bBlock BehaviorsBlockModel
		if diags := data.Config.Behaviors.As(ctx, &bBlock, basetypes.ObjectAsOptions{}); !diags.HasError() {
			if !bBlock.Custom.IsNull() && !bBlock.Custom.IsUnknown() {
				var behaviors []BehaviorModel
				resp.Diagnostics.Append(bBlock.Custom.ElementsAs(ctx, &behaviors, false)...)
				if !resp.Diagnostics.HasError() {
					for i, b := range behaviors {
						prefix := fmt.Sprintf("behaviors.custom[%d]", i)
						for _, msg := range ValidateBehaviorModel(&b, prefix) {
							resp.Diagnostics.AddAttributeError(
								path.Root("config").AtName("behaviors").AtName("custom"),
								"Invalid behavior condition",
								msg,
							)
						}
					}
				}
			}
		}
	}

	// --- Domain mappings must reference a known origin name ---
	// Collect origin names defined in config.origins.
	originNames := map[string]struct{}{}
	if !data.Config.Origins.IsNull() && !data.Config.Origins.IsUnknown() {
		var origins []OriginModel
		if diags := data.Config.Origins.ElementsAs(ctx, &origins, false); !diags.HasError() {
			for _, o := range origins {
				if !o.Name.IsNull() && !o.Name.IsUnknown() {
					originNames[o.Name.ValueString()] = struct{}{}
				}
			}
		}
	}
	// Only validate when we have at least one origin defined (skip on unknown/partial configs).
	if len(originNames) > 0 && data.Config.Domains != nil {
		for di, domain := range *data.Config.Domains {
			for mi, mapping := range domain.Mappings {
				tm := mapping.TargetMapping.ValueString()
				if mapping.TargetMapping.IsNull() || mapping.TargetMapping.IsUnknown() || tm == "" {
					continue
				}
				// Mappings pointing at an origin_set are validated separately (or by the backend).
				if mapping.TargetType.ValueString() == "origin_set" {
					continue
				}
				if _, ok := originNames[tm]; !ok {
					resp.Diagnostics.AddAttributeError(
						path.Root("config").AtName("domains").AtListIndex(di).AtName("mappings").AtListIndex(mi).AtName("target_mapping"),
						"Unknown origin reference",
						fmt.Sprintf("domains[%d].mappings[%d].target_mapping %q does not match any origin name defined in config.origins.", di, mi, tm),
					)
				}
			}
		}
	}

	// --- origin_sets: each set must have at least 2 origins ---
	if data.Config.OriginSets != nil {
		for i, os := range data.Config.OriginSets {
			if len(os.Origins) < 2 {
				resp.Diagnostics.AddAttributeError(
					path.Root("config").AtName("origin_sets").AtListIndex(i).AtName("origins"),
					"Origin set requires at least 2 origins",
					fmt.Sprintf("origin_sets[%d] (%q) must have at least 2 origins for failover, but has %d.",
						i, os.Name.ValueString(), len(os.Origins)),
				)
			}
		}
	}

	// --- stream_logs.log_destination must reference a known log destination name ---
	// Collect log destination names from config.log_destinations.
	logDestNames := map[string]struct{}{}
	if data.Config.LogDestinations != nil {
		for _, ld := range *data.Config.LogDestinations {
			if !ld.Name.IsNull() && !ld.Name.IsUnknown() {
				logDestNames[ld.Name.ValueString()] = struct{}{}
			}
		}
	}
	if len(logDestNames) > 0 && !data.Config.Behaviors.IsNull() && !data.Config.Behaviors.IsUnknown() {
		var bBlock BehaviorsBlockModel
		if diags := data.Config.Behaviors.As(ctx, &bBlock, basetypes.ObjectAsOptions{}); !diags.HasError() {
			// Check default behavior.
			if !bBlock.Default.IsNull() && !bBlock.Default.IsUnknown() {
				var defB DefaultBehaviorModel
				if diags := bBlock.Default.As(ctx, &defB, basetypes.ObjectAsOptions{}); !diags.HasError() {
					if defB.Actions != nil && defB.Actions.StreamLogs != nil {
						name := defB.Actions.StreamLogs.UnifiedLogDestination.ValueString()
						if name != "" {
							if _, ok := logDestNames[name]; !ok {
								resp.Diagnostics.AddAttributeError(
									path.Root("config").AtName("behaviors").AtName("default").AtName("actions").AtName("stream_logs").AtName("log_destination"),
									"Unknown log destination reference",
									fmt.Sprintf("behaviors.default.actions.stream_logs.log_destination %q does not match any name defined in config.log_destinations.", name),
								)
							}
						}
					}
				}
			}
			// Check custom behaviors.
			if !bBlock.Custom.IsNull() && !bBlock.Custom.IsUnknown() {
				var customs []BehaviorModel
				if diags := bBlock.Custom.ElementsAs(ctx, &customs, false); !diags.HasError() {
					for i, b := range customs {
						if b.Actions == nil || b.Actions.StreamLogs == nil {
							continue
						}
						name := b.Actions.StreamLogs.UnifiedLogDestination.ValueString()
						if name == "" {
							continue
						}
						if _, ok := logDestNames[name]; !ok {
							resp.Diagnostics.AddAttributeError(
								path.Root("config").AtName("behaviors").AtName("custom").AtListIndex(i).AtName("actions").AtName("stream_logs").AtName("log_destination"),
								"Unknown log destination reference",
								fmt.Sprintf("behaviors.custom[%d].actions.stream_logs.log_destination %q does not match any name defined in config.log_destinations.", i, name),
							)
						}
					}
				}
			}
		}
	}
}
