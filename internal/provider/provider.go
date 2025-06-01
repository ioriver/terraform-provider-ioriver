package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
)

// Ensure IORiverProvider satisfies various provider interfaces.
var _ provider.Provider = &IORiverProvider{}

// IORiverProvider defines the provider implementation.
type IORiverProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// IORiverProviderModel describes the provider data model.
type IORiverProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *IORiverProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ioriver"
	resp.Version = p.version
}

func (p *IORiverProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The IO River provider is used for managing resources supported by IO River. The provider needs to be configured with the proper API token before it can be used.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "IO River management endpoint URL",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "IO River API token",
				Optional:            true,
			},
		},
	}
}

func (p *IORiverProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data IORiverProviderModel

	endpoint := os.Getenv(APIEndpointEnvVar)
	apiToken := os.Getenv(APITokenEvnVar)

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Endpoint.ValueString() != "" {
		endpoint = data.Endpoint.ValueString()
	}
	if data.Token.ValueString() != "" {
		apiToken = data.Token.ValueString()
	}

	if endpoint == "" {
		endpoint = "https://manage.ioriver.io/api/v1/"
	}

	tflog.Info(ctx, fmt.Sprintf("IORiver version: %s", p.version))

	// client configuration for data sources and resources
	client := ioriver.NewClient(apiToken)
	client.EndpointUrl = endpoint
	client.TerraformVersion = p.version
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *IORiverProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCertificateResource,
		NewAccountProviderResource,
		NewServiceResource,
		NewDomainResource,
		NewOriginResource,
		NewOriginShieldResource,
		NewServiceProviderResource,
		NewTrafficPolicyResource,
		NewBehaviorResource,
		NewComputeResource,
		NewHealthMonitorResource,
		NewPerformanceMonitorResource,
		NewProtocolConfigResource,
		NewLogDestinationResource,
		NewUrlSigningKeyResource,
	}
}

func (p *IORiverProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &IORiverProvider{
			version: version,
		}
	}
}
