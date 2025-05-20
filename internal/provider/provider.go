// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	// "github.com/hashicorp/terraform-plugin-framework/ephemeral"
	// "github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	// "github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure scorecardProvider satisfies various provider interfaces.
var(
	_ provider.Provider = &scorecardProvider{}
	// _ provider.ProviderWithFunctions = &scorecardProvider{}
	// _ provider.ProviderWithEphemeralResources = &scorecardProvider{}
) 

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &scorecardProvider{
			version: version,
		}
	}
}

// scorecardProvider defines the provider implementation.
type scorecardProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	client *http.Client
	token string
	version string
}

// scorecardProviderModel describes the provider data model.
type scorecardProviderModel struct {
	ApiToken types.String `tfsdk:"api_token"`
}

func (p *scorecardProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "scorecard"
	resp.Version = p.version
}

func (p *scorecardProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
    resp.Schema = schema.Schema{
        Attributes: map[string]schema.Attribute{
            "api_token": schema.StringAttribute{
                Description: "DX Web API token for authentication.",
                Required:    true,
                Sensitive:   true,
            },
        },
    }
}

func (p *scorecardProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
    var config scorecardProviderModel

    // Load provider config
    diags := req.Config.Get(ctx, &config)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() {
        return
    }

    token := config.ApiToken.ValueString()

    if token == "" {
        resp.Diagnostics.AddError(
            "Missing API Token",
            "The provider could not retrieve an API token. This is required to authenticate with the DX API.",
        )
        return
    }

    // Initialize HTTP client
    client := &http.Client{}

    // Save for use in resources
    p.client = client
    p.token = token
}

func (p *scorecardProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewScorecardResource,
	}
}

func (p *scorecardProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *scorecardProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewScorecardDataSource,
	}
}

func (p *scorecardProvider) Functions(ctx context.Context) []func() function.Function {
	return nil
}