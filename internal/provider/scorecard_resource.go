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
	"github.com/hashicorp/terraform-plugin-framework/validator"
	"github.com/hashicorp/terraform-plugin-framework/validator/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/validator/numbervalidator"
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
	EvaluationFrequency 			types.Number `tfsdk:"evaluation_frequency"`
	
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
	OutputCustomOptions types.String `tfsdk:"output_custom_options"` //TODO figure out how to model this

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

type scorecardCreateResponse struct {
	Ok bool `json:"ok"`
	Scorecard scorecardModel `json:"scorecard"`
}


func (r *scorecardResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scorecard"
}

func (r *scorecardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DX Scorecard.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique ID of the scorecard.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the scorecard.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				  },
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The type of scorecard. Options: 'LEVEL', 'POINTS'.",
				Validators: []validator.String{
					stringvalidator.OneOf("LEVEL", "POINTS"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				  },
				
			},
			"entity_filter_type": schema.StringAttribute{
				Required:    true,
				Description: "The filtering strategy when deciding what entities this scorecard should assess. Options: 'entity_types', 'sql'",
				Validators: []validator.String{
					stringvalidator.OneOf("entity_types", "sql"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				  },
			},
			"evaluation_frequency": schema.NumberAttribute{
				Required:    true,
				Description: "How often the scorecard is evaluated (in hours). [2|4|8|24]",
				Validators: []validator.Number{
					numbervalidator.OneOf(2, 4, 8, 24),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				  },
			},

			// Conditionally required for levels-based scorecards
			"empty_level_label": schema.StringAttribute{
				Optional:    true,
				Description: "The label to display when an entity has not achieved any levels in the scorecard (levels scorecards only).",
			},
			"empty_level_color": schema.StringAttribute{
				Optional:    true,
				Description: "The color hex code to display when an entity has not achieved any levels in the scorecard (levels scorecards only).",
			},
			"levels": schema.ListNestedAttribute{
				Optional:    true,
				Description: "The levels that can be achieved in this scorecard (levels scorecards only).",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":   schema.StringAttribute{Required: true},
						"id":    schema.StringAttribute{Computed: true},
						"name":  schema.StringAttribute{Required: true},
						"color": schema.StringAttribute{Required: true},
						"rank":  schema.NumberAttribute{Required: true},
					},
				},
			},

			// Conditionally required for points-based scorecards
			"check_groups": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Groups of checks, to help organize the scorecard for entity owners (points scorecards only).",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key":      schema.StringAttribute{Required: true},
						"id":       schema.StringAttribute{Computed: true},
						"name":     schema.StringAttribute{Required: true},
						"ordering": schema.NumberAttribute{Required: true},
					},
				},
			},

			// Optional metadata
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the scorecard.",
			},
			"published": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the scorecard is published.",
			},
			"entity_filter_type_identifiers": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "List of entity type identifiers that the scorecard should run against.",
			},
			"entity_filter_sql": schema.StringAttribute{
				Optional:    true,
				Description: "Custom SQL used to filter entities that the scorecard should run against.",
			},

			// For now, all check field are required. This may change in the future.
			"checks": schema.ListNestedAttribute{
				Optional:    true,
				Description: "List of checks that are applied to entities in the scorecard.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":               schema.StringAttribute{Computed: true},
						"name":             schema.StringAttribute{Required: true},
						"description":      schema.StringAttribute{Required: true},
						"ordering":         schema.NumberAttribute{Required: true},
						"sql":              schema.StringAttribute{Required: true},
						"filter_sql":       schema.StringAttribute{Required: true},
						"filter_message":   schema.StringAttribute{Required: true},
						"output_enabled":   schema.BoolAttribute{Required: true},
						"output_type":      schema.StringAttribute{Required: true},
						"output_aggregation": schema.StringAttribute{Required: true},
						"output_custom_options": schema.StringAttribute{Required: true}, // JSON string (you may eventually want to use a map)
						"estimated_dev_days":    schema.NumberAttribute{Required: true},
						"external_url":          schema.StringAttribute{Required: true},
						"published":             schema.BoolAttribute{Required: true},

						// Fields for level-based scorecards
						"scorecard_level_key": schema.StringAttribute{Optional: true},
						"level": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"key":   schema.StringAttribute{Required: true},
								"id":    schema.StringAttribute{Computed: true},
								"name":  schema.StringAttribute{Required: true},
								"color": schema.StringAttribute{Required: true},
								"rank":  schema.NumberAttribute{Required: true},
							},
						},

						// Fields for points-based scorecards
						"scorecard_check_group_key": schema.StringAttribute{Optional: true},
						"check_group": schema.SingleNestedAttribute{
							Optional: true,
							Description: "Optional check group. If provided, all its fields (except 'id') are required.",
							Attributes: map[string]schema.Attribute{
								"key":      schema.StringAttribute{Required: true},
								"id":       schema.StringAttribute{Computed: true},
								"name":     schema.StringAttribute{Required: true},
								"ordering": schema.NumberAttribute{Required: true},
							},
						},
						"points": schema.NumberAttribute{Optional: true},
					},
				},
			},
		},
	}
}


// func (r *scorecardResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
// 	resp.Schema = schema.Schema{
// 		// This description is used by the documentation generator and the language server.
// 		MarkdownDescription: "Manages a DX Scorecard",

// 		Attributes: map[string]schema.Attribute{
// 			"configurable_attribute": schema.StringAttribute{
// 				MarkdownDescription: "Example configurable attribute",
// 				Optional:            true,
// 			},
// 			"defaulted": schema.StringAttribute{
// 				MarkdownDescription: "Example configurable attribute with default value",
// 				Optional:            true,
// 				Computed:            true,
// 				Default:             stringdefault.StaticString("example value when not configured"),
// 			},
// 			"id": schema.StringAttribute{
// 				Computed:            true,
// 				MarkdownDescription: "Example identifier",
// 				PlanModifiers: []planmodifier.String{
// 					stringplanmodifier.UseStateForUnknown(),
// 				},
// 			},
// 		},
// 	}
// }

func (r *scorecardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan scorecardModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate required fields for CREATE endpoint
	if plan.Name.IsNull() || plan.Name.IsUnknown() {
		resp.Diagnostics.AddError("Missing required field", "The 'name' field must be specified.")
		return
	}
	if plan.Type.IsNull() || plan.Type.IsUnknown() {
		resp.Diagnostics.AddError("Missing required field", "The 'type' field must be specified.")
		return
	}
	if plan.EntityFilterType.IsNull() || plan.EntityFilterType.IsUnknown() {
		resp.Diagnostics.AddError("Missing required field", "The 'entity_filter_type' field must be specified.")
		return
	}
	if plan.EvaluationFrequency.IsNull() || plan.EvaluationFrequency.IsUnknown() {
		resp.Diagnostics.AddError("Missing required field", "The 'evaluation_frequency_hours' field must be specified.")
		return
	}

	// Validate required fields based on scorecard type
	scorecardType := plan.Type.ValueString()
	switch scorecardType {
	case "LEVEL":
		if plan.EmptyLevelLabel.IsNull() || plan.EmptyLevelLabel.IsUnknown() {
			resp.Diagnostics.AddError("Missing required field", "The 'empty_level_label' field must be specified for LEVEL scorecards.")
		}
		if plan.EmptyLevelColor.IsNull() || plan.EmptyLevelColor.IsUnknown() {
			resp.Diagnostics.AddError("Missing required field", "The 'empty_level_color' field must be specified for LEVEL scorecards.")
		}
		if plan.Levels == nil || len(plan.Levels) == 0 {
			resp.Diagnostics.AddError("Missing required field", "At least one 'level' must be specified for LEVEL scorecards.")
		}
	case "POINTS":
		if plan.CheckGroups == nil || len(plan.CheckGroups) == 0 {
			resp.Diagnostics.AddError("Missing required field", "At least one 'check_group' must be specified for POINTS scorecards.")
		}
	default:
		resp.Diagnostics.AddError("Invalid scorecard type", fmt.Sprintf("Unsupported scorecard type: %s", scorecardType))
	}

	// If there are any errors above, return immediately.
	if resp.Diagnostics.HasError() {
		return
	}

	// Construct API request payload
	payload := map[string]interface{}{
		// Required fields
		"name":                 plan.Name.ValueString(),
		"type":                 scorecardType,
		"entity_filter_type":   plan.EntityFilterType.ValueString(),
		"evaluation_frequency": int(plan.EvaluationFrequency.ValueInt64()), // 2,4,8,24 values enforced by validator above
	}

	// Add LEVEL-specific required fields
	if scorecardType == "LEVEL" {
		payload["empty_level_label"] = plan.EmptyLevelLabel.ValueString()
		payload["empty_level_color"] = plan.EmptyLevelColor.ValueString()

		levels := []map[string]interface{}{}
		for _, level := range plan.Levels {
			levels = append(levels, map[string]interface{}{
				"key":   level.Key.ValueString(),
				"name":  level.Name.ValueString(),
				"color": level.Color.ValueString(),
				"rank":  int(level.Rank.ValueInt64()),
			})
		}
		payload["levels"] = levels
	}

	// Add POINTS-specific required fields
	if scorecardType == "POINTS" {
		checkGroups := []map[string]interface{}{}
		for _, group := range plan.CheckGroups {
			checkGroups = append(checkGroups, map[string]interface{}{
				"key":      group.Key.ValueString(),
				"name":     group.Name.ValueString(),
				"ordering": int(group.Ordering.ValueInt64()),
			})
		}
		payload["check_groups"] = checkGroups
	}

	// Add optional fields if they're present
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload["description"] = plan.Description.ValueString()
	}
	if !plan.Published.IsNull() && !plan.Published.IsUnknown() {
		payload["published"] = plan.Published.ValueBool()
	}
	if plan.EntityFilterTypeIdentifiers != nil && len(plan.EntityFilterTypeIdentifiers) > 0 {
		identifiers := make([]string, 0, len(plan.EntityFilterTypeIdentifiers))
		for _, id := range plan.EntityFilterTypeIdentifiers {
			if !id.IsNull() && !id.IsUnknown() {
				identifiers = append(identifiers, id.ValueString())
			}
		}
		payload["entity_filter_type_identifiers"] = identifiers
	}
	if !plan.EntityFilterSql.IsNull() && !plan.EntityFilterSql.IsUnknown() {
		payload["entity_filter_sql"] = plan.EntityFilterSql.ValueString()
	}

	// Add checks
	checks := []map[string]interface{}{}
	for _, check := range plan.Checks {
		checkPayload := map[string]interface{}{
			"name":                 check.Name.ValueString(),
			"description":          check.Description.ValueString(),
			"ordering":             int(check.Ordering.ValueInt64()),
			"sql":                  check.Sql.ValueString(),
			"filter_sql":           check.FilterSql.ValueString(),
			"filter_message":       check.FilterMessage.ValueString(),
			"output_enabled":       check.OutputEnabled.ValueBool(),
			"output_type":          check.OutputType.ValueString(),
			"output_aggregation":   check.OutputAggregation.ValueString(),
			"output_custom_options": check.OutputCustomOptions.ValueString(),
			"estimated_dev_days":   int(check.EstimatedDevDays.ValueInt64()),
			"external_url":         check.ExternalUrl.ValueString(),
			"published":            check.Published.ValueBool(),
		}

		// Add LEVEL-specific check fields
		if scorecardType == "LEVEL" {
			checkPayload["scorecard_level_key"] = check.ScorecardLevelKey.ValueString()
			checkPayload["level"] = map[string]interface{}{
				"key":   check.Level.Key.ValueString(),
				"name":  check.Level.Name.ValueString(),
				"color": check.Level.Color.ValueString(),
				"rank":  int(check.Level.Rank.ValueInt64()),
			}
		}

		// Add POINTS-specific check fields
		if scorecardType == "POINTS" {
			checkPayload["scorecard_check_group_key"] = check.ScorecardCheckGroupKey.ValueString()
			checkPayload["check_group"] = map[string]interface{}{
				"key":      check.CheckGroup.Key.ValueString(),
				"name":     check.CheckGroup.Name.ValueString(),
				"ordering": int(check.CheckGroup.Ordering.ValueInt64()),
			}
			checkPayload["points"] = int(check.Points.ValueInt64())
		}

		checks = append(checks, checkPayload)
	}
	payload["checks"] = checks

	// Marshal payload to JSON
	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error marshaling request payload", err.Error())
		return
	}

	// Send API request
	apiURL := fmt.Sprintf("%s/scorecards", r.client.BaseURL)
	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		resp.Diagnostics.AddError("Error creating HTTP request", err.Error())
		return
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	reqHTTP.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.client.APIKey))

	httpResp, err := r.client.HTTPClient.Do(reqHTTP)
	if err != nil {
		resp.Diagnostics.AddError("Error sending HTTP request", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("Status code: %d", httpResp.StatusCode))
		return
	}

	// Decode response
	var apiResponse scorecardCreateResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		resp.Diagnostics.AddError("Error decoding API response", err.Error())
		return
	}

	// Set Scorecard ID
	plan.Id = types.StringValue(apiResponse.Scorecard.Id)

	// TODO - Are names the best way to match here? I don't think so....
	// Set Level IDs
	for i := range plan.Levels {
		for _, apiResponseLevel := range apiResponse.Scorecard.Levels {
			if plan.Levels[i].Name.ValueString() == apiResponseLevel.Name {
				plan.Levels[i].Id = types.StringValue(apiResponseLevel.Id)
				break
			}
		}
	}

	// Set Check IDs
	for i := range plan.Checks {
		for _, apiResponseCheck := range apiResponse.Scorecard.Checks {
			if plan.Checks[i].Name.ValueString() == apiResponseCheck.Name {
				plan.Checks[i].Id = types.StringValue(apiResponseCheck.Id)
				break
			}
		}
	}

	// Set Check Group IDs	
	for i := range plan.CheckGroups {
		for _, apiResponseCheckGroup := range apiResponse.Scorecard.CheckGroups {
			if plan.CheckGroups[i].Name.ValueString() == apiResponseCheckGroup.Name {
				plan.CheckGroups[i].Id = types.StringValue(apiResponseCheckGroup.Id)
				break
			}
		}
	}


	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}


// func (r *scorecardResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
// 	// Prevent panic if the provider has not been configured.
// 	if req.ProviderData == nil {
// 		return
// 	}

// 	provider, ok := req.ProviderData.(*scorecardProvider)

// 	if !ok {
// 		resp.Diagnostics.AddError(
// 			"Unexpected Resource Configure Type",
// 			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
// 		)

// 		return
// 	}

// 	r.client = provider.client
// 	r.token = provider.token

// }

// func (r *scorecardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
//     var data scorecardModel

//     // Read config from Terraform
//     diags := req.Config.Get(ctx, &data)
//     resp.Diagnostics.Append(diags...)
//     if resp.Diagnostics.HasError() {
//         return
//     }

//     // Convert to API payload
//     payload := scorecardAPI{
//         Name:        data.Name.ValueString(),
//         Description: data.Description.ValueString(),
//     }

//     for _, c := range data.Checks {
//         payload.Checks = append(payload.Checks, checkAPI{
//             Name:  c.Name.ValueString(),
//             Query: c.Query.ValueString(),
//         })
//     }

//     // Encode to JSON
//     body, err := json.Marshal(payload)
//     if err != nil {
//         resp.Diagnostics.AddError("Error marshaling JSON", err.Error())
//         return
//     }

//     // Make the POST request
//     reqUrl := "https://api.getdx.com/scorecards"
//     httpReq, err := http.NewRequestWithContext(ctx, "POST", reqUrl, bytes.NewBuffer(body))
//     if err != nil {
//         resp.Diagnostics.AddError("Error building request", err.Error())
//         return
//     }

//     httpReq.Header.Set("Authorization", "Bearer "+r.token)
//     httpReq.Header.Set("Content-Type", "application/json")

//     httpResp, err := r.client.Do(httpReq)
//     if err != nil {
//         resp.Diagnostics.AddError("Error making API request", err.Error())
//         return
//     }
//     defer httpResp.Body.Close()

//     if httpResp.StatusCode != 201 {
//         resp.Diagnostics.AddError(
//             "Failed to create scorecard",
//             fmt.Sprintf("Status: %d", httpResp.StatusCode),
//         )
//         return
//     }

//     // Parse the response
//     var result scorecardAPI
//     if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
//         resp.Diagnostics.AddError("Error decoding API response", err.Error())
//         return
//     }

//     // Save the ID and update state
//     data.ID = types.StringValue(result.ID)
//     resp.State.Set(ctx, &data)
// }


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
