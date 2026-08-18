package provider

import (
	"context"
	"testing"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_billing_explainer"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
)

func money(amount, currency string) models.BillingExplainerMoney {
	return models.BillingExplainerMoney{Amount: amount, Currency: currency}
}

func lineItem(costType, amount, currency string) models.BillingExplainerCostLineItem {
	return models.BillingExplainerCostLineItem{
		CostType: costType,
		Cost:     money(amount, currency),
	}
}

// TestMapBillingExplainerToModel_FullyPopulated exercises every nested
// mapping path (payers map, account/service double-maps, all three summary
// siblings with populated cost-line-item lists, support charges with a
// populated details list) against the real shape captured from the test
// tenant's 2025-12 response.
func TestMapBillingExplainerToModel_FullyPopulated(t *testing.T) {
	ctx := context.Background()

	awsSummary := models.BillingExplainerServiceSummary{
		ServiceCharges: []models.BillingExplainerCostLineItem{lineItem("usage", "937.78", "USD")},
		Discounts:      []models.BillingExplainerCostLineItem{lineItem("sppDiscount", "-65.64", "USD")},
		Tax:            []models.BillingExplainerCostLineItem{},
		SupportCharges: models.BillingExplainerSupportCharges{
			Cost: money("0.00", "USD"),
			Details: []models.BillingExplainerSupportDetail{
				{
					BaseCost:           money("10.00", "USD"),
					Cost:               money("8.00", "USD"),
					Description:        "Enterprise Support",
					ProjectId:          "proj-1",
					ServiceDescription: "Support",
				},
			},
		},
		Credits:      []models.BillingExplainerCostLineItem{},
		Savings:      []models.BillingExplainerCostLineItem{},
		OtherCharges: []models.BillingExplainerCostLineItem{},
		Refunds:      []models.BillingExplainerCostLineItem{},
		Total:        money("872.14", "USD"),
	}

	doitSummary := models.BillingExplainerServiceSummary{
		ServiceCharges: []models.BillingExplainerCostLineItem{lineItem("usage", "937.78", "USD")},
		Discounts:      []models.BillingExplainerCostLineItem{},
		Tax:            []models.BillingExplainerCostLineItem{},
		SupportCharges: models.BillingExplainerSupportCharges{Cost: money("0.00", "USD"), Details: []models.BillingExplainerSupportDetail{}},
		Credits:        []models.BillingExplainerCostLineItem{},
		Savings:        []models.BillingExplainerCostLineItem{lineItem("flexsaveSavings", "0.00", "USD")},
		OtherCharges:   []models.BillingExplainerCostLineItem{},
		Refunds:        []models.BillingExplainerCostLineItem{},
		Total:          money("937.78", "USD"),
	}

	awsWithoutDoitSummary := models.BillingExplainerServiceSummary{
		ServiceCharges: []models.BillingExplainerCostLineItem{lineItem("usage", "937.78", "USD")},
		Discounts:      []models.BillingExplainerCostLineItem{lineItem("bundledDiscount", "0.00", "USD")},
		Tax:            []models.BillingExplainerCostLineItem{},
		SupportCharges: models.BillingExplainerSupportCharges{Cost: money("0.00", "USD"), Details: []models.BillingExplainerSupportDetail{}},
		Credits:        []models.BillingExplainerCostLineItem{lineItem("credit", "0.00", "USD")},
		Savings:        []models.BillingExplainerCostLineItem{lineItem("savingsPlanNegation", "0.00", "USD")},
		OtherCharges:   []models.BillingExplainerCostLineItem{lineItem("ocbCharges", "0.00", "USD")},
		Refunds:        []models.BillingExplainerCostLineItem{lineItem("refund", "0.00", "USD")},
		Total:          money("937.78", "USD"),
	}

	updateTime, err := time.Parse(time.RFC3339, "2026-08-13T09:08:12Z")
	if err != nil {
		t.Fatalf("failed to parse fixture time: %v", err)
	}

	apiResp := &models.BillingExplainerCustomer{
		CustomerId:         "JhV7WydpOlW8DeVRVVNf",
		InvoiceMonth:       "2025-12",
		UpdateTime:         updateTime,
		DoitCredits:        money("0.00", "USD"),
		InvoiceAdjustments: money("0.00", "USD"),
		Payers: map[string]models.BillingExplainerPayer{
			"630855515779": {
				PayerId:      "630855515779",
				FriendlyName: "DoiT Reseller Account #3303",
				Summary: models.BillingExplainerSummary{
					Aws:            awsSummary,
					Doit:           doitSummary,
					AwsWithoutDoit: awsWithoutDoitSummary,
				},
				Account: models.BillingExplainerCostDifferences{
					"458867540890": {
						Doit: models.BillingExplainerCostDetail{"usage": money("935.50", "USD")},
						Aws:  models.BillingExplainerCostDetail{"sppDiscount": money("-65.48", "USD"), "usage": money("935.50", "USD")},
					},
				},
				Service: models.BillingExplainerCostDifferences{
					"Amazon Virtual Private Cloud": {
						Doit: models.BillingExplainerCostDetail{"usage": money("935.49", "USD")},
						Aws:  models.BillingExplainerCostDetail{"sppDiscount": money("-65.48", "USD"), "usage": money("935.49", "USD")},
					},
				},
			},
		},
	}

	var data billingExplainerDataSourceModel
	diags := mapBillingExplainerToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := data.CustomerId.ValueString(); got != "JhV7WydpOlW8DeVRVVNf" {
		t.Errorf("customer_id = %q, want JhV7WydpOlW8DeVRVVNf", got)
	}
	if got := data.InvoiceMonth.ValueString(); got != "2025-12" {
		t.Errorf("invoice_month = %q, want 2025-12", got)
	}
	if got := data.UpdateTime.ValueString(); got != "2026-08-13T09:08:12Z" {
		t.Errorf("update_time = %q, want 2026-08-13T09:08:12Z", got)
	}
	if got := data.DoitCredits.Amount.ValueString(); got != "0.00" {
		t.Errorf("doit_credits.amount = %q, want 0.00", got)
	}

	payerElems := data.Payers.Elements()
	if len(payerElems) != 1 {
		t.Fatalf("payers has %d elements, want 1", len(payerElems))
	}
	payer, ok := payerElems["630855515779"].(datasource_billing_explainer.PayersValue)
	if !ok {
		t.Fatalf("payer element has type %T, want PayersValue", payerElems["630855515779"])
	}
	if got := payer.FriendlyName.ValueString(); got != "DoiT Reseller Account #3303" {
		t.Errorf("payers[630855515779].friendly_name = %q, want DoiT Reseller Account #3303", got)
	}

	awsWithoutDoit := payer.Summary.AwsWithoutDoit
	credits := awsWithoutDoit.Credits.Elements()
	if len(credits) != 1 {
		t.Fatalf("summary.aws_without_doit.credits has %d elements, want 1", len(credits))
	}
	creditItem, ok := credits[0].(datasource_billing_explainer.CreditsValue)
	if !ok {
		t.Fatalf("credits[0] has type %T, want CreditsValue", credits[0])
	}
	if got := creditItem.CostType.ValueString(); got != "credit" {
		t.Errorf("credits[0].cost_type = %q, want credit", got)
	}

	supportDetails := awsWithoutDoit.SupportCharges.Details.Elements()
	if len(supportDetails) != 0 {
		t.Errorf("aws_without_doit.support_charges.details has %d elements, want 0", len(supportDetails))
	}

	awsDetails := payer.Summary.Aws.SupportCharges.Details.Elements()
	if len(awsDetails) != 1 {
		t.Fatalf("aws.support_charges.details has %d elements, want 1", len(awsDetails))
	}
	detail, ok := awsDetails[0].(datasource_billing_explainer.DetailsValue)
	if !ok {
		t.Fatalf("details[0] has type %T, want DetailsValue", awsDetails[0])
	}
	if got := detail.ServiceDescription.ValueString(); got != "Support" {
		t.Errorf("details[0].service_description = %q, want Support", got)
	}

	accountElems := payer.Account.Elements()
	if len(accountElems) != 1 {
		t.Fatalf("account has %d elements, want 1", len(accountElems))
	}
	account, ok := accountElems["458867540890"].(datasource_billing_explainer.AccountValue)
	if !ok {
		t.Fatalf("account element has type %T, want AccountValue", accountElems["458867540890"])
	}
	awsMoney := account.Aws.Elements()
	if len(awsMoney) != 2 {
		t.Fatalf("account[458867540890].aws has %d elements, want 2", len(awsMoney))
	}
	usage, ok := awsMoney["usage"].(datasource_billing_explainer.AwsValue)
	if !ok {
		t.Fatalf("account.aws[usage] has type %T, want AwsValue", awsMoney["usage"])
	}
	if got := usage.Amount.ValueString(); got != "935.50" {
		t.Errorf("account[458867540890].aws[usage].amount = %q, want 935.50", got)
	}
}

// TestMapBillingExplainerToModel_EmptyCollections confirms every list/map
// field defaults to an empty (never null) collection when the API returns no
// entries, per the Computed-Only List Attributes rule.
func TestMapBillingExplainerToModel_EmptyCollections(t *testing.T) {
	ctx := context.Background()

	emptySummary := models.BillingExplainerServiceSummary{
		ServiceCharges: []models.BillingExplainerCostLineItem{},
		Discounts:      []models.BillingExplainerCostLineItem{},
		Tax:            []models.BillingExplainerCostLineItem{},
		SupportCharges: models.BillingExplainerSupportCharges{Cost: money("0.00", "USD"), Details: []models.BillingExplainerSupportDetail{}},
		Credits:        []models.BillingExplainerCostLineItem{},
		Savings:        []models.BillingExplainerCostLineItem{},
		OtherCharges:   []models.BillingExplainerCostLineItem{},
		Refunds:        []models.BillingExplainerCostLineItem{},
		Total:          money("0.00", "USD"),
	}

	apiResp := &models.BillingExplainerCustomer{
		CustomerId:         "cust-1",
		InvoiceMonth:       "2026-01",
		UpdateTime:         time.Now(),
		DoitCredits:        money("0.00", "USD"),
		InvoiceAdjustments: money("0.00", "USD"),
		Payers: map[string]models.BillingExplainerPayer{
			"payer-1": {
				PayerId:      "payer-1",
				FriendlyName: "Empty Payer",
				Summary: models.BillingExplainerSummary{
					Aws:            emptySummary,
					Doit:           emptySummary,
					AwsWithoutDoit: emptySummary,
				},
				Account: models.BillingExplainerCostDifferences{},
				Service: models.BillingExplainerCostDifferences{},
			},
		},
	}

	var data billingExplainerDataSourceModel
	diags := mapBillingExplainerToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	payer, ok := data.Payers.Elements()["payer-1"].(datasource_billing_explainer.PayersValue)
	if !ok {
		t.Fatalf("payer element has type %T, want PayersValue", data.Payers.Elements()["payer-1"])
	}

	if payer.Account.IsNull() {
		t.Error("account should be an empty map, not null")
	}
	if len(payer.Account.Elements()) != 0 {
		t.Errorf("account has %d elements, want 0", len(payer.Account.Elements()))
	}
	if payer.Service.IsNull() {
		t.Error("service should be an empty map, not null")
	}
	if payer.Summary.Aws.Credits.IsNull() {
		t.Error("summary.aws.credits should be an empty list, not null")
	}
	if len(payer.Summary.Aws.Credits.Elements()) != 0 {
		t.Errorf("summary.aws.credits has %d elements, want 0", len(payer.Summary.Aws.Credits.Elements()))
	}
	if payer.Summary.Aws.SupportCharges.Details.IsNull() {
		t.Error("summary.aws.support_charges.details should be an empty list, not null")
	}
}

// TestMapBillingExplainerToModel_NoPayers confirms an empty payers map
// resolves to an empty (never null) map at the root.
func TestMapBillingExplainerToModel_NoPayers(t *testing.T) {
	ctx := context.Background()

	apiResp := &models.BillingExplainerCustomer{
		CustomerId:         "cust-1",
		InvoiceMonth:       "2026-01",
		UpdateTime:         time.Now(),
		DoitCredits:        money("0.00", "USD"),
		InvoiceAdjustments: money("0.00", "USD"),
		Payers:             map[string]models.BillingExplainerPayer{},
	}

	var data billingExplainerDataSourceModel
	diags := mapBillingExplainerToModel(ctx, apiResp, &data)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Payers.IsNull() {
		t.Error("payers should be an empty map, not null")
	}
	if len(data.Payers.Elements()) != 0 {
		t.Errorf("payers has %d elements, want 0", len(data.Payers.Elements()))
	}
}
