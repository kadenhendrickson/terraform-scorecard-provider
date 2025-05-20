// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &scorecardResource{}
// var _ resource.ResourceWithImportState = &scorecardResource{}

func NewscorecardResource() resource.Resource {
	return &scorecardResource{}
}

// scorecardResource defines the resource implementation.
type scorecardResource struct {
	client *http.Client
	token string
}

// scorecardModel describes the resource data model.
type scorecardModel struct {
	// Required fields
    Id          				types.String `tfsdk:"id"`
    Name        				types.String `tfsdk:"name"`
	Type        				types.String `tfsdk:"type"`
	EntityFilterType 			types.String `tfsdk:"entity_filter_type"`
	EvalutionFrequency 			types.Number `tfsdk:"evaluation_frequency"`
	
	// Conditionally required fields for levels based scorecards
	EmptyLevelLabel 			types.String `tfsdk:"empty_level_label"`
	EmptyLevelColor 			types.String `tfsdk:"empty_level_color"`
	Levels      				[]levelModel `tfsdk:"levels"`

	// Conditionally required fields for points based scorecards
	CheckGroups 				[]checkGroupModel `tfsdk:"check_groups"`

	// Optional fields
    Description 				types.String `tfsdk:"description"`
	Published      				types.Bool `tfsdk:"published"`
	EntityFilterTypeIdentifiers []types.String `tfsdk:"entity_filter_type_identifiers"`
	EntityFilterSql 			types.String `tfsdk:"entity_filter_sql"`
    Checks      				[]checkModel `tfsdk:"checks"`
}

type levelModel struct {
	Key 	types.String `tfsdk:"key"`
	Id  	types.String `tfsdk:"id"`
	Name  	types.String `tfsdk:"name"`
	Color 	types.String `tfsdk:"color"`
	Rank  	types.Number `tfsdk:"rank"`
}

type checkGroupModel struct {
	Key 		types.String `tfsdk:"key"`
	Id  		types.String `tfsdk:"id"`
	Name  		types.String `tfsdk:"name"`
	Ordering 	types.Number `tfsdk:"ordering"`
}

type checkModel struct {
	Id 				types.String `tfsdk:"id"`
	Name 			types.String `tfsdk:"name"`
	Description 	types.String `tfsdk:"description"`
	Ordering 		types.Number `tfsdk:"ordering"`
	Sql 			types.String `tfsdk:"sql"`
	FilterSql 		types.String `tfsdk:"filter_sql"`
	FilterMessage 	types.String `tfsdk:"filter_message"`
	OutputEnabled 	types.Bool `tfsdk:"output_enabled"`
	
	OutputType 			types.String `tfsdk:"output_type"`
	OutputAggregation 	types.String `tfsdk:"output_aggregation"`
	OutputCustomOptions types.String `tfsdk:"output_custom_options"`

	EstimatedDevDays 	types.Number `tfsdk:"estimated_dev_days"`
	ExternalUrl			types.String `tfsdk:"external_url"`
	Published 			types.Bool `tfsdk:"published"`

	// Additional fields for level based scorecards
	ScorecardLevelKey 	types.String `tfsdk:"scorecard_level_key"`
	Level 				levelModel `tfsdk:"level"`

	// Additional fields for points based scorecards
	ScorecardCheckGroupKey 	types.String `tfsdk:"scorecard_check_group_key"`
	CheckGroup 				checkGroupModel `tfsdk:"check_group"`
	Points 					types.Number `tfsdk:"points"`
}


func (r *scorecardResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_example"
}

func (r *scorecardResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"configurable_attribute": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Optional:            true,
			},
			"defaulted": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("example value when not configured"),
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *scorecardResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*scorecardProvider)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = provider.client
	r.token = provider.token

}

func (r *scorecardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var data scorecardModel

    // Read config from Terraform
    diags := req.Config.Get(ctx, &data)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() {
        return
    }

    // Convert to API payload
    payload := scorecardAPI{
        Name:        data.Name.ValueString(),
        Description: data.Description.ValueString(),
    }

    for _, c := range data.Checks {
        payload.Checks = append(payload.Checks, checkAPI{
            Name:  c.Name.ValueString(),
            Query: c.Query.ValueString(),
        })
    }

    // Encode to JSON
    body, err := json.Marshal(payload)
    if err != nil {
        resp.Diagnostics.AddError("Error marshaling JSON", err.Error())
        return
    }

    // Make the POST request
    reqUrl := "https://api.getdx.com/scorecards"
    httpReq, err := http.NewRequestWithContext(ctx, "POST", reqUrl, bytes.NewBuffer(body))
    if err != nil {
        resp.Diagnostics.AddError("Error building request", err.Error())
        return
    }

    httpReq.Header.Set("Authorization", "Bearer "+r.token)
    httpReq.Header.Set("Content-Type", "application/json")

    httpResp, err := r.client.Do(httpReq)
    if err != nil {
        resp.Diagnostics.AddError("Error making API request", err.Error())
        return
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != 201 {
        resp.Diagnostics.AddError(
            "Failed to create scorecard",
            fmt.Sprintf("Status: %d", httpResp.StatusCode),
        )
        return
    }

    // Parse the response
    var result scorecardAPI
    if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
        resp.Diagnostics.AddError("Error decoding API response", err.Error())
        return
    }

    // Save the ID and update state
    data.ID = types.StringValue(result.ID)
    resp.State.Set(ctx, &data)
}


func (r *scorecardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data scorecardModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *scorecardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data scorecardModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *scorecardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data scorecardModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete example, got error: %s", err))
	//     return
	// }
}

func (r *scorecardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
