package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ServiceProviderResource{}
var _ resource.ResourceWithImportState = &ServiceProviderResource{}
var _ resource.ResourceWithUpgradeState = &ServiceProviderResource{}

func NewServiceProviderResource() resource.Resource {
	return &ServiceProviderResource{}
}

type ServiceProviderResourceId struct {
	serviceProviderId string
	serviceId         string
}

type ServiceProviderResource struct {
	client *ioriver.IORiverClient
}

type ServiceProviderResourceModelV0 struct {
	Id                 types.String `tfsdk:"id"`
	Service            types.String `tfsdk:"service"`
	AccountProvider    types.String `tfsdk:"account_provider"`
	IsUnmanaged        types.Bool   `tfsdk:"is_unmanaged"`
	CName              types.String `tfsdk:"cname"`
	DisplayName        types.String `tfsdk:"display_name"`
	ProviderCustomData types.String `tfsdk:"provider_custom_data"`
	IsFailed           types.Bool   `tfsdk:"is_failed"`
	Status             types.String `tfsdk:"status"`
	StatusDetails      types.String `tfsdk:"status_details"`
	Restored           types.Bool   `tfsdk:"restored"`
	Name               types.String `tfsdk:"name"`
}

type ServiceProviderResourceModelV1 struct {
	Id                 types.String `tfsdk:"id"`
	Service            types.String `tfsdk:"service"`
	AccountProvider    types.String `tfsdk:"account_provider"`
	IsUnmanaged        types.Bool   `tfsdk:"is_unmanaged"`
	CName              types.String `tfsdk:"cname"`
	DisplayName        types.String `tfsdk:"display_name"`
	ProviderCustomData types.Object `tfsdk:"provider_custom_data"`
	IsFailed           types.Bool   `tfsdk:"is_failed"`
	Status             types.String `tfsdk:"status"`
	StatusDetails      types.String `tfsdk:"status_details"`
	Restored           types.Bool   `tfsdk:"restored"`
	Name               types.String `tfsdk:"name"`
}

func (r *ServiceProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_provider"
}

var schemaV0 = schema.Schema{
	MarkdownDescription: "Service Provider resource",

	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "ServiceProvider identifier",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service": schema.StringAttribute{
			MarkdownDescription: "The id of the service this service provider belongs to",
			Required:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"account_provider": schema.StringAttribute{
			MarkdownDescription: "The account provider to be assigned to this service",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"is_unmanaged": schema.BoolAttribute{
			MarkdownDescription: "Is this an unmanaged ServiceProvider, which means that the provider is not managed by IO River and will not be configured automatically. This is a write-only field required during creation of the service provider",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false), // has default since this is a write-only field
			PlanModifiers: []planmodifier.Bool{
				boolplanmodifier.RequiresReplace(),
			},
		},
		"cname": schema.StringAttribute{
			MarkdownDescription: "CName of the ServiceProvider",
			Optional:            true,
			Computed:            true,
		},
		"display_name": schema.StringAttribute{
			MarkdownDescription: "Display name of the ServiceProvider",
			Optional:            true,
			Computed:            true,
		},
		"provider_custom_data": schema.StringAttribute{
			MarkdownDescription: "ServiceProvider custom data in JSON format. This is a write-only field used to pass provider-specific information during creation or update of the service provider",
			Optional:            true,
			Computed:            true,
		},
		"is_failed": schema.BoolAttribute{
			MarkdownDescription: "An indicator of whether the ServiceProvider is in a failed state",
			Computed:            true,
		},
		"status": schema.StringAttribute{
			MarkdownDescription: "ServiceProvider status, e.g., Active, Deploying, etc.",
			Computed:            true,
		},
		"status_details": schema.StringAttribute{
			MarkdownDescription: "ServiceProvider detailed status, providing additional information about the current status of the ServiceProvider",
			Computed:            true,
		},
		"restored": schema.BoolAttribute{
			MarkdownDescription: "Is ServiceProvider restored",
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the provider, e.g. Fastly, Cloudflare, etc.",
			Computed:            true,
		},
	},
}

var schemaV1 = schema.Schema{
	Version:             1,
	MarkdownDescription: "Service Provider resource",
	Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "ServiceProvider identifier",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"service": schema.StringAttribute{
			MarkdownDescription: "The id of the service this service provider belongs to",
			Required:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"account_provider": schema.StringAttribute{
			MarkdownDescription: "The account provider to be assigned to this service",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"is_unmanaged": schema.BoolAttribute{
			MarkdownDescription: "Is this an unmanaged ServiceProvider, which means that the provider is not managed by IO River and will not be configured automatically. This is a write-only field required during creation of the service provider",
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false), // has default since this is a write-only field
			PlanModifiers: []planmodifier.Bool{
				boolplanmodifier.RequiresReplace(),
			},
		},
		"cname": schema.StringAttribute{
			MarkdownDescription: "CName of the ServiceProvider",
			Optional:            true,
			Computed:            true,
		},
		"display_name": schema.StringAttribute{
			MarkdownDescription: "Display name of the ServiceProvider",
			Optional:            true,
			Computed:            true,
		},
		"provider_custom_data": schema.SingleNestedAttribute{
			MarkdownDescription: "ServiceProvider custom data. " +
				"This is a write-only field used to pass provider-specific information during creation or update of the service provider",
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"fastly": schema.StringAttribute{
					MarkdownDescription: "Custom data in JSON format, normalized to string",
					Optional:            true,
					Sensitive:           true,
					CustomType:          jsontypes.NormalizedType{},
					Validators: []validator.String{
						stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("akamai")),
					},
				},
				"akamai": schema.SingleNestedAttribute{
					MarkdownDescription: "Akamai product information",
					Optional:            true,
					Validators: []validator.Object{
						objectvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("fastly")),
					},
					Attributes: akamaiSchemaAttrs,
				},
			},
		},
		"is_failed": schema.BoolAttribute{
			MarkdownDescription: "An indicator of whether the ServiceProvider is in a failed state",
			Computed:            true,
		},
		"status": schema.StringAttribute{
			MarkdownDescription: "ServiceProvider status, e.g., Active, Deploying, etc.",
			Computed:            true,
		},
		"status_details": schema.StringAttribute{
			MarkdownDescription: "ServiceProvider detailed status, providing additional information about the current status of the ServiceProvider",
			Computed:            true,
		},
		"restored": schema.BoolAttribute{
			MarkdownDescription: "Is ServiceProvider restored",
			Computed:            true,
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the provider, e.g. Fastly, Cloudflare, etc.",
			Computed:            true,
		},
	},
}

// The akamai provider_custom_data block. Adding or renaming a field means
// updating akamaiSchemaAttrs, akamaiAttrTypes, and akamaiHCLToWireKey so they
// stay in sync. Keep the schema-side names as the map keys everywhere; the
// wire (backend JSON) names appear only in akamaiHCLToWireKey.

var akamaiSchemaAttrs = map[string]schema.Attribute{
	"property_group": schema.StringAttribute{MarkdownDescription: "Akamai Property Group", Required: true},
	"contract_id":    schema.StringAttribute{MarkdownDescription: "Akamai Contract ID", Required: true},
	"product":        schema.StringAttribute{MarkdownDescription: "Product within the Property Group", Required: true},
	"cp_code":        schema.StringAttribute{MarkdownDescription: "Content provider code", Required: true},
	"stream_type":    schema.StringAttribute{MarkdownDescription: "Stream type - only for relevant products", Optional: true},
}

var akamaiAttrTypes = map[string]attr.Type{
	"property_group": types.StringType,
	"contract_id":    types.StringType,
	"product":        types.StringType,
	"cp_code":        types.StringType,
	"stream_type":    types.StringType,
}

// akamaiHCLToWireKey maps schema attribute names to backend JSON keys.
var akamaiHCLToWireKey = map[string]string{
	"property_group": "group_id",
	"contract_id":    "contract_id",
	"product":        "product_id",
	"cp_code":        "cp_code",
	"stream_type":    "stream_type",
}

var akamaiObjType = types.ObjectType{AttrTypes: akamaiAttrTypes}

var providerCustomDataAttrTypesV1 = map[string]attr.Type{
	"fastly": jsontypes.NormalizedType{},
	"akamai": akamaiObjType,
}

// akamaiHCLToWire converts the HCL akamai Object into the backend JSON map.
// Null/unknown fields are omitted; the schema's Required flag already rejects
// missing required inputs before we get here.
func akamaiHCLToWire(obj types.Object) (map[string]string, error) {
	out := make(map[string]string, len(akamaiHCLToWireKey))
	for hcl, v := range obj.Attributes() {
		wireKey, ok := akamaiHCLToWireKey[hcl]
		if !ok {
			continue
		}
		s, ok := v.(types.String)
		if !ok {
			return nil, fmt.Errorf("akamai attribute %q has unexpected type %T", hcl, v)
		}
		if s.IsNull() || s.IsUnknown() {
			continue
		}
		out[wireKey] = s.ValueString()
	}
	return out, nil
}

// akamaiWireToHCL converts the backend JSON map into an HCL akamai Object.
// Wire keys that are absent from the payload become null in the HCL object.
func akamaiWireToHCL(wire map[string]string) (types.Object, error) {
	values := make(map[string]attr.Value, len(akamaiHCLToWireKey))
	for hcl, wireKey := range akamaiHCLToWireKey {
		if s, ok := wire[wireKey]; ok {
			values[hcl] = types.StringValue(s)
		} else {
			values[hcl] = types.StringNull()
		}
	}
	obj, diags := types.ObjectValue(akamaiAttrTypes, values)
	if diags.HasError() {
		return types.ObjectNull(akamaiAttrTypes), fmt.Errorf("failed to build akamai object: %v", diags)
	}
	return obj, nil
}

func (r *ServiceProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schemaV1
}

func (r *ServiceProviderResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeV0ToV1,
		},
	}
}

// Configure resource and retrieve API client
func (r *ServiceProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create ServiceProvider resource
func (r *ServiceProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceProviderResourceModelV1

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newData := resourceCreate(r.client, ctx, req, resp, r, data, false)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read ServiceProvider resource
func (r *ServiceProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceProviderResourceModelV1

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update ServiceProvider resource
func (r *ServiceProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServiceProviderResourceModelV1

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)

}

// Delete ServiceProvider resource
func (r *ServiceProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceProviderResourceModelV1

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import ServiceProvider resource
func (r *ServiceProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (ServiceProviderResource) create(ctx context.Context, client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	newSp, error := client.CreateServiceProvider(newObj.(ioriver.ServiceProvider))
	if error != nil {
		return newSp, error
	}

	// Wait for the service provider to become active
	// If we don't wait and will try to create a traffic policy, it will fail on validation
	// This operation is performed under the global lock, so it blocks other resources creation.
	timeout := 60 * time.Minute
	interval := 10 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		newSp, error = client.GetServiceProvider(newSp.Service, newSp.Id)
		if error == nil {
			tflog.Info(ctx, fmt.Sprintf("Current Serivce-Provider status: %s", newSp.Status))
			if newSp.Status == "Active" {
				break
			}
		}

		if time.Now().After(deadline) {
			break
		}
		time.Sleep(interval)
	}

	return newSp, error
}

func (ServiceProviderResource) read(ctx context.Context, client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(ServiceProviderResourceId)
	return client.GetServiceProvider(resourceId.serviceId, resourceId.serviceProviderId)
}

func (ServiceProviderResource) update(ctx context.Context, client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateServiceProvider(obj.(ioriver.ServiceProvider))
}

func (ServiceProviderResource) delete(ctx context.Context, client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(ServiceProviderResourceId)
	return client.DeleteServiceProvider(resourceId.serviceId, resourceId.serviceProviderId, "disconnect")
}

func (ServiceProviderResource) getId(data interface{}) interface{} {
	d := data.(ServiceProviderResourceModelV1)
	serviceProviderId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return ServiceProviderResourceId{serviceProviderId, serviceId}
}

// Convert ServiceProvider resource to ServiceProvider API object
func (r ServiceProviderResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(ServiceProviderResourceModelV1)

	providerName := d.Name.ValueString()
	if providerName == "" {
		var err error
		providerName, err = getProviderName(d.AccountProvider.ValueString(), r.client)
		if err != nil {
			return ServiceProviderResourceModelV1{}, err
		}
	}

	providerHCLName, err := normalizeProviderName(providerName)
	if err != nil {
		return ioriver.ServiceProvider{}, err
	}

	providerCustomData, err := translateProviderCustomDataToAPI(ctx, &d, providerHCLName)
	if err != nil {
		return ioriver.ServiceProvider{}, err
	}

	return ioriver.ServiceProvider{
		Id:                 d.Id.ValueString(),
		Service:            d.Service.ValueString(),
		AccountProvider:    d.AccountProvider.ValueString(),
		IsUnmanaged:        d.IsUnmanaged.ValueBool(),
		CName:              d.CName.ValueString(),
		DisplayName:        d.DisplayName.ValueString(),
		ProviderCustomData: providerCustomData,
		IsFailed:           d.IsFailed.ValueBool(),
		Status:             d.Status.ValueString(),
		StatusDetails:      d.StatusDetails.ValueString(),
		Restored:           d.Restored.ValueBool(),
		Name:               d.Name.ValueString(),
	}, nil
}

// Convert ServiceProvider API object to ServiceProvider resource
func (ServiceProviderResource) objToResource(ctx context.Context, obj interface{}, data interface{}) (interface{}, error) {
	serviceProvider := obj.(*ioriver.ServiceProvider)

	providerCustomDataModel, err := translateProviderCustomDataFromAPI(ctx, serviceProvider)
	if err != nil {
		return ServiceProviderResourceModelV1{}, err
	}

	return ServiceProviderResourceModelV1{
		Id:                 types.StringValue(serviceProvider.Id),
		Service:            types.StringValue(serviceProvider.Service),
		AccountProvider:    types.StringValue(serviceProvider.AccountProvider),
		IsUnmanaged:        types.BoolValue(serviceProvider.IsUnmanaged),
		CName:              types.StringValue(serviceProvider.CName),
		DisplayName:        types.StringValue(serviceProvider.DisplayName),
		ProviderCustomData: providerCustomDataModel,
		IsFailed:           types.BoolValue(serviceProvider.IsFailed),
		Status:             types.StringValue(serviceProvider.Status),
		StatusDetails:      types.StringValue(serviceProvider.StatusDetails),
		Restored:           types.BoolValue(serviceProvider.Restored),
		Name:               types.StringValue(serviceProvider.Name),
	}, nil
}

// Upgrade provider_custom_data from V0 to V1.
func upgradeV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var oldState ServiceProviderResourceModelV0

	// Fetch the data using the legacy V0 struct layout
	resp.Diagnostics.Append(req.State.Get(ctx, &oldState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if oldState.ProviderCustomData.IsUnknown() {
		resp.Diagnostics.AddError(
			"State Upgrade Failure: Unknown Legacy Data",
			"The state upgrader encountered an unknown value for 'provider_custom_data' in legacy state.",
		)
		return
	}

	// Erase any old data of v0 present and warn the user that it will be erased.
	if !oldState.ProviderCustomData.IsNull() && oldState.ProviderCustomData.ValueString() != "" {
		warning := "The state upgrader encountered legacy 'provider_custom_data' in the state. This data will be erased."
		resp.Diagnostics.AddWarning(
			"State Upgrade Warning: Legacy Data",
			warning,
		)
	}

	// Set in state the new model.
	nullValueCustomDataNew := types.ObjectNull(providerCustomDataAttrTypesV1)

	newState := ServiceProviderResourceModelV1{
		Id:                 oldState.Id,
		Service:            oldState.Service,
		AccountProvider:    oldState.AccountProvider,
		IsUnmanaged:        oldState.IsUnmanaged,
		CName:              oldState.CName,
		DisplayName:        oldState.DisplayName,
		ProviderCustomData: nullValueCustomDataNew,
		IsFailed:           oldState.IsFailed,
		Status:             oldState.Status,
		StatusDetails:      oldState.StatusDetails,
		Restored:           oldState.Restored,
		Name:               oldState.Name,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func translateProviderCustomDataToAPI(ctx context.Context, spModel *ServiceProviderResourceModelV1, providerName string) (string, error) {
	providerCustomData := spModel.ProviderCustomData
	if providerCustomData.IsNull() || providerCustomData.IsUnknown() {
		return "", nil
	}

	if providerName == "" {
		return "", fmt.Errorf("provider name is required when provider custom data is set")
	}

	attrs := providerCustomData.Attributes()

	isFastlySet := false
	if fastlyAttr, exists := attrs["fastly"]; exists {
		isFastlySet = !fastlyAttr.IsNull() && !fastlyAttr.IsUnknown()
	}

	isAkamaiSet := false
	if akamaiAttr, exists := attrs["akamai"]; exists {
		isAkamaiSet = !akamaiAttr.IsNull() && !akamaiAttr.IsUnknown()
	}

	if !isFastlySet && !isAkamaiSet {
		return "", nil
	}

	if isFastlySet && isAkamaiSet {
		return "", fmt.Errorf("provider custom data supports only one provider block")
	}

	// Fastly
	if isFastlySet {
		if providerName != "fastly" {
			return "", fmt.Errorf("provider custom data set for fastly, but provider is %s", providerName)
		}

		fastlyVal, ok := attrs["fastly"].(jsontypes.Normalized)
		if !ok {
			return "", fmt.Errorf("failed to read fastly provider custom data as normalized JSON, got %T", attrs["fastly"])
		}
		return fastlyVal.ValueString(), nil
	}

	// Akamai
	if isAkamaiSet {
		if providerName != "akamai" {
			return "", fmt.Errorf("provider custom data set for akamai, but provider is %s", providerName)
		}

		akamaiObj, ok := attrs["akamai"].(types.Object)
		if !ok {
			return "", fmt.Errorf("failed to parse akamai provider custom data, got %T", attrs["akamai"])
		}

		payload, err := akamaiHCLToWire(akamaiObj)
		if err != nil {
			return "", err
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("failed to marshal akamai provider custom data: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}

func normalizeProviderName(providerName string) (string, error) {
	if providerName == "" {
		return "", fmt.Errorf("provider name cannot be empty")
	}

	if providerHCLName, exists := ProviderNamesMapBackendToHCL[providerName]; exists {
		return providerHCLName, nil
	}

	if _, exists := ProviderNamesMapHCLToBackend[providerName]; exists {
		return providerName, nil
	}

	return "", fmt.Errorf("unknown provider name: %s", providerName)
}

func translateProviderCustomDataFromAPI(_ context.Context, serviceProvider *ioriver.ServiceProvider) (types.Object, error) {
	providerCustomData := serviceProvider.ProviderCustomData
	if providerCustomData == "" {
		return types.ObjectNull(providerCustomDataAttrTypesV1), nil
	}

	providerHCLName, err := normalizeProviderName(serviceProvider.Name)
	if err != nil {
		return types.ObjectNull(providerCustomDataAttrTypesV1), err
	}

	switch providerHCLName {
	case "fastly":
		fastlyValue := jsontypes.NewNormalizedValue(providerCustomData)
		finalObj, diags := types.ObjectValue(providerCustomDataAttrTypesV1, map[string]attr.Value{
			"fastly": fastlyValue,
			"akamai": types.ObjectNull(akamaiObjType.AttrTypes),
		})
		if diags.HasError() {
			return types.ObjectNull(providerCustomDataAttrTypesV1), fmt.Errorf("failed to create provider custom data object: %v", diags)
		}
		return finalObj, nil
	case "akamai":
		var wire map[string]string
		if err := json.Unmarshal([]byte(providerCustomData), &wire); err != nil {
			return types.ObjectNull(providerCustomDataAttrTypesV1), fmt.Errorf("failed to parse akamai provider custom data: %v", err)
		}
		akamaiValue, err := akamaiWireToHCL(wire)
		if err != nil {
			return types.ObjectNull(providerCustomDataAttrTypesV1), err
		}

		finalObj, diags := types.ObjectValue(providerCustomDataAttrTypesV1, map[string]attr.Value{
			"fastly": jsontypes.NewNormalizedNull(),
			"akamai": akamaiValue,
		})
		if diags.HasError() {
			return types.ObjectNull(providerCustomDataAttrTypesV1), fmt.Errorf("failed to create provider custom data object: %v", diags)
		}
		return finalObj, nil
	default:
		return types.ObjectNull(providerCustomDataAttrTypesV1), fmt.Errorf("unknown custom data for provider: %s", providerHCLName)
	}
}

func getProviderName(AccountProviderId string, client *ioriver.IORiverClient) (string, error) {
	accountProvider, err := client.GetAccountProvider(AccountProviderId)
	if err != nil {
		return "", fmt.Errorf("failed to load account_provider %q: %w", AccountProviderId, err)
	}
	return accountProvider.Details.Name, nil
}
