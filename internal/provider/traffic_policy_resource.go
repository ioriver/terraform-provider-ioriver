package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TrafficPolicyResource{}
var _ resource.ResourceWithImportState = &TrafficPolicyResource{}

func NewTrafficPolicyResource() resource.Resource {
	return &TrafficPolicyResource{}
}

type TrafficPolicyResourceId struct {
	trafficPolicyId string
	serviceId       string
}

type TrafficPolicyResource struct {
	client *ioriver.IORiverClient
}

type ProviderResourceModel struct {
	ServiceProvider types.String `tfsdk:"service_provider"`
	Weight          types.Int64  `tfsdk:"weight"`
}

type GeoResourceModel struct {
	Continent   types.String `tfsdk:"continent"`
	Country     types.String `tfsdk:"country"`
	Subdivision types.String `tfsdk:"subdivision"`
}

type PolicyHealthMonitorResourceModel struct {
	HealthMonitor types.String `tfsdk:"health_monitor"`
}

type PolicyPerformanceMonitorResourceModel struct {
	PerformanceMonitor types.String `tfsdk:"performance_monitor"`
}

type TrafficPolicyResourceModel struct {
	Id                  types.String                            `tfsdk:"id"`
	Service             types.String                            `tfsdk:"service"`
	Type                types.String                            `tfsdk:"type"`
	Failover            types.Bool                              `tfsdk:"failover"`
	IsDefault           types.Bool                              `tfsdk:"is_default"`
	Providers           []ProviderResourceModel                 `tfsdk:"providers"`
	Geos                []GeoResourceModel                      `tfsdk:"geos"`
	HealthMonitors      []PolicyHealthMonitorResourceModel      `tfsdk:"health_monitors"`
	PerformanceMonitors []PolicyPerformanceMonitorResourceModel `tfsdk:"performance_monitors"`
}

func (r *TrafficPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_traffic_policy"
}

func (r *TrafficPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "TrafficPolicy resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "TrafficPolicy identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "The id of the service this TrafficPolicy belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "TrafficPolicy type",
				Required:            true,
			},
			"failover": schema.BoolAttribute{
				MarkdownDescription: "Is automatic failover enabled",
				Required:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Is is the default TrafficPolicy",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"providers": schema.SetNestedAttribute{
				MarkdownDescription: "List of service provider within this policy",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"service_provider": schema.StringAttribute{
							MarkdownDescription: "Service provider Id",
							Required:            true,
						},
						"weight": schema.Int64Attribute{
							MarkdownDescription: "Service provider weight",
							Required:            true,
						},
					},
				},
			},
			"geos": schema.SetNestedAttribute{
				MarkdownDescription: "List of geos to apply this policy on",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"continent": schema.StringAttribute{
							MarkdownDescription: "Name of continent",
							Optional:            true,
							Computed:            true,
						},
						"country": schema.StringAttribute{
							MarkdownDescription: "Name of country",
							Optional:            true,
							Computed:            true,
						},
						"subdivision": schema.StringAttribute{
							MarkdownDescription: "Name of subdivision (state)",
							Optional:            true,
							Computed:            true,
						},
					},
				},
			},
			"health_monitors": schema.SetNestedAttribute{
				MarkdownDescription: "TrafficPolicy list of health monitors",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"health_monitor": schema.StringAttribute{
							MarkdownDescription: "Health-monitor Id",
							Required:            true,
						},
					},
				},
			},
			"performance_monitors": schema.SetNestedAttribute{
				MarkdownDescription: "TrafficPolicy list of performance monitors",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"performance_monitor": schema.StringAttribute{
							MarkdownDescription: "Performance-monitor Id",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

// Configure resource and retrieve API client
func (r *TrafficPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client := ConfigureBase(ctx, req, resp)
	if client == nil {
		return
	}
	r.client = client
}

// Create TrafficPolicy resource
func (r *TrafficPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TrafficPolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	newData := resourceCreate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Read TrafficPolicy resource
func (r *TrafficPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TrafficPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	newData := resourceRead(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Update TrafficPolicy resource
func (r *TrafficPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TrafficPolicyResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	newData := resourceUpdate(r.client, ctx, req, resp, r, data)
	if newData == nil {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newData)...)
}

// Delete TrafficPolicy resource
func (r *TrafficPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TrafficPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceDelete(r.client, ctx, req, resp, r, data)
}

// Import TrafficPolicy resource
func (r *TrafficPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	serviceResourceImport(ctx, req, resp)
}

// ------- Implement base Resource API ---------

func (TrafficPolicyResource) create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error) {
	return client.CreateTrafficPolicy(newObj.(ioriver.TrafficPolicy))
}

func (TrafficPolicyResource) read(client *ioriver.IORiverClient, id interface{}) (interface{}, error) {
	resourceId := id.(TrafficPolicyResourceId)
	return client.GetTrafficPolicy(resourceId.serviceId, resourceId.trafficPolicyId)
}

func (TrafficPolicyResource) update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error) {
	return client.UpdateTrafficPolicy(obj.(ioriver.TrafficPolicy))
}

func (TrafficPolicyResource) delete(client *ioriver.IORiverClient, id interface{}) error {
	resourceId := id.(TrafficPolicyResourceId)
	return client.DeleteTrafficPolicy(resourceId.serviceId, resourceId.trafficPolicyId)
}

func (TrafficPolicyResource) getId(data interface{}) interface{} {
	d := data.(TrafficPolicyResourceModel)
	trafficPolicyId := d.Id.ValueString()
	serviceId := d.Service.ValueString()
	return TrafficPolicyResourceId{trafficPolicyId, serviceId}
}

// Convert TrafficPolicy resource to TrafficPolicy API object
func (TrafficPolicyResource) resourceToObj(ctx context.Context, data interface{}) (interface{}, error) {
	d := data.(TrafficPolicyResourceModel)

	// convert providers
	trafficPolicyProviders := []ioriver.TrafficPolicyProvider{}
	for _, provider := range d.Providers {
		trafficPolicyProviders = append(trafficPolicyProviders,
			ioriver.TrafficPolicyProvider{
				ServiceProvider: provider.ServiceProvider.ValueString(),
				Weight:          int(provider.Weight.ValueInt64()),
			})
	}

	// convert geos
	trafficPolicyGeos := []ioriver.TrafficPolicyGeo{}
	for _, geo := range d.Geos {
		trafficPolicyGeos = append(trafficPolicyGeos,
			ioriver.TrafficPolicyGeo{
				Continent:   geo.Continent.ValueString(),
				Country:     geo.Country.ValueString(),
				Subdivision: geo.Subdivision.ValueString(),
			})
	}

	// convert health-checks
	trafficPolicyHealthChecks := []ioriver.TrafficPolicyHealthCheck{}
	for _, healthCheck := range d.HealthMonitors {
		trafficPolicyHealthChecks = append(trafficPolicyHealthChecks,
			ioriver.TrafficPolicyHealthCheck{
				HealthCheck: healthCheck.HealthMonitor.ValueString(),
			})
	}

	// convert performance-checks
	trafficPolicyPerfChecks := []ioriver.TrafficPolicyPerfCheck{}
	for _, perfCheck := range d.PerformanceMonitors {
		trafficPolicyPerfChecks = append(trafficPolicyPerfChecks,
			ioriver.TrafficPolicyPerfCheck{
				PerformanceCheck: perfCheck.PerformanceMonitor.ValueString(),
			})
	}

	return ioriver.TrafficPolicy{
		Id:           d.Id.ValueString(),
		Service:      d.Service.ValueString(),
		Type:         d.Type.ValueString(),
		Failover:     d.Failover.ValueBool(),
		IsDefault:    d.IsDefault.ValueBool(),
		Providers:    trafficPolicyProviders,
		Geos:         trafficPolicyGeos,
		HealthChecks: trafficPolicyHealthChecks,
		PerfChecks:   trafficPolicyPerfChecks,
	}, nil
}

// Convert TrafficPolicy API object to TrafficPolicy resource
func (TrafficPolicyResource) objToResource(ctx context.Context, obj interface{}) (interface{}, error) {
	trafficPolicy := obj.(*ioriver.TrafficPolicy)

	// convert providers
	modelProviders := []ProviderResourceModel{}
	for _, provider := range trafficPolicy.Providers {
		modelProviders = append(modelProviders,
			ProviderResourceModel{
				ServiceProvider: types.StringValue(provider.ServiceProvider),
				Weight:          types.Int64Value(int64(provider.Weight)),
			})
	}

	// convert geos
	modelGeos := []GeoResourceModel{}
	for _, geo := range trafficPolicy.Geos {
		modelGeos = append(modelGeos,
			GeoResourceModel{
				Continent:   types.StringValue(geo.Continent),
				Country:     types.StringValue(geo.Country),
				Subdivision: types.StringValue(geo.Subdivision),
			})
	}

	// convert health-checks
	modelHealthChecks := []PolicyHealthMonitorResourceModel{}
	for _, healthCheck := range trafficPolicy.HealthChecks {
		modelHealthChecks = append(modelHealthChecks,
			PolicyHealthMonitorResourceModel{
				HealthMonitor: types.StringValue(healthCheck.HealthCheck),
			})
	}

	// convert perf-checks
	modelPerfChecks := []PolicyPerformanceMonitorResourceModel{}
	for _, perfCheck := range trafficPolicy.PerfChecks {
		modelPerfChecks = append(modelPerfChecks,
			PolicyPerformanceMonitorResourceModel{
				PerformanceMonitor: types.StringValue(perfCheck.PerformanceCheck),
			})
	}

	return TrafficPolicyResourceModel{
		Id:                  types.StringValue(trafficPolicy.Id),
		Service:             types.StringValue(trafficPolicy.Service),
		Type:                types.StringValue(trafficPolicy.Type),
		Failover:            types.BoolValue(trafficPolicy.Failover),
		IsDefault:           types.BoolValue(trafficPolicy.IsDefault),
		Providers:           modelProviders,
		Geos:                modelGeos,
		HealthMonitors:      modelHealthChecks,
		PerformanceMonitors: modelPerfChecks,
	}, nil
}
