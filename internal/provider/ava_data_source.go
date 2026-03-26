package provider

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*avaDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*avaDataSource)(nil)

func NewAvaDataSource() datasource.DataSource {
	return &avaDataSource{}
}

type avaDataSource struct {
	client *models.ClientWithResponses
}

type avaDataSourceModel struct {
	Id       types.String `tfsdk:"id"`
	Question types.String `tfsdk:"question"`
	Answer   types.String `tfsdk:"answer"`
}

func (d *avaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ava"
}

func (d *avaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Ask Ava, DoiT's AI-powered cloud assistant, a question and get a response. " +
			"Each invocation makes a synchronous API call that typically takes 15-30 seconds. " +
			"Conversations are ephemeral (not persisted) to avoid orphaned server-side state.",
		MarkdownDescription: "Ask [Ava](https://www.doit.com/ava/), DoiT's AI-powered cloud assistant, a question and get a response.\n\n" +
			"> **Note:** Each invocation makes a synchronous API call that typically takes 15-30 seconds. " +
			"Responses are non-deterministic — the same question may yield different answers on each run. " +
			"Conversations are ephemeral (not persisted) to avoid orphaned server-side state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "A deterministic hash of the question, used as the data source identifier.",
				MarkdownDescription: "A deterministic hash of the question, used as the data source identifier.",
			},
			"question": schema.StringAttribute{
				Required:            true,
				Description:         "The question to ask Ava.",
				MarkdownDescription: "The question to ask Ava.",
			},
			"answer": schema.StringAttribute{
				Computed:            true,
				Description:         "The Ava response text.",
				MarkdownDescription: "The Ava response text.",
			},
		},
	}
}

func (d *avaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*models.ClientWithResponses)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *models.ClientWithResponses, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *avaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data avaDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	question := data.Question.ValueString()

	// Always use ephemeral mode to avoid creating persistent conversations
	ephemeral := true
	apiResp, err := d.client.AskAvaSyncWithResponse(ctx, models.AskAvaSyncJSONRequestBody{
		Question:  question,
		Ephemeral: &ephemeral,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Querying Ava",
			fmt.Sprintf("Unable to send question to Ava: %v", err),
		)
		return
	}

	if apiResp.StatusCode() != 200 || apiResp.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Error Querying Ava",
			fmt.Sprintf("Ava API returned status %d: %s", apiResp.StatusCode(), string(apiResp.Body)),
		)
		return
	}

	// Set a deterministic ID based on the question hash
	hash := sha256.Sum256([]byte(question))
	data.Id = types.StringValue(fmt.Sprintf("%x", hash[:8]))
	data.Answer = types.StringValue(apiResp.JSON200.Answer)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
