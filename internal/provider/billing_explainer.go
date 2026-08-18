package provider

import (
	"context"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/datasource_billing_explainer"
	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mapBillingExplainerToModel maps a GetBillingExplainerPerPayer API response
// onto the billing_explainer data source model.
func mapBillingExplainerToModel(ctx context.Context, apiResp *models.BillingExplainerCustomer, data *billingExplainerDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.CustomerId = types.StringValue(apiResp.CustomerId)
	data.InvoiceMonth = types.StringValue(apiResp.InvoiceMonth)
	data.UpdateTime = types.StringValue(apiResp.UpdateTime.UTC().Format(time.RFC3339))

	doitCredits, d := datasource_billing_explainer.NewDoitCreditsValue(
		datasource_billing_explainer.DoitCreditsValue{}.AttributeTypes(ctx),
		moneyAttrs(apiResp.DoitCredits.Amount, apiResp.DoitCredits.Currency),
	)
	diags.Append(d...)
	data.DoitCredits = doitCredits

	invoiceAdjustments, d := datasource_billing_explainer.NewInvoiceAdjustmentsValue(
		datasource_billing_explainer.InvoiceAdjustmentsValue{}.AttributeTypes(ctx),
		moneyAttrs(apiResp.InvoiceAdjustments.Amount, apiResp.InvoiceAdjustments.Currency),
	)
	diags.Append(d...)
	data.InvoiceAdjustments = invoiceAdjustments

	payers, d := mapPayersMap(ctx, apiResp.Payers)
	diags.Append(d...)
	data.Payers = payers

	return diags
}

// buildObjectMap maps a Go map of API items into a Terraform map of a
// generated nested-object value type, defaulting to an empty (never null)
// map when there are no entries, per the Computed-Only List Attributes rule
// (the same rule applies to maps).
func buildObjectMap[T any, V attr.Value](elemType attr.Type, attrTypes map[string]attr.Type, src map[string]T, attrsFor func(T) (map[string]attr.Value, diag.Diagnostics), newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(src) == 0 {
		m, d := types.MapValue(elemType, map[string]attr.Value{})
		diags.Append(d...)
		return m, diags
	}

	vals := make(map[string]attr.Value, len(src))
	for k, v := range src {
		a, d := attrsFor(v)
		diags.Append(d...)
		val, d := newFn(attrTypes, a)
		diags.Append(d...)
		vals[k] = val
	}

	m, d := types.MapValue(elemType, vals)
	diags.Append(d...)
	return m, diags
}

func mapPayersMap(ctx context.Context, payers map[string]models.BillingExplainerPayer) (types.Map, diag.Diagnostics) {
	return buildObjectMap(
		datasource_billing_explainer.PayersValue{}.Type(ctx),
		datasource_billing_explainer.PayersValue{}.AttributeTypes(ctx),
		payers,
		func(p models.BillingExplainerPayer) (map[string]attr.Value, diag.Diagnostics) {
			return payerAttrs(ctx, p)
		},
		datasource_billing_explainer.NewPayersValue,
	)
}

func payerAttrs(ctx context.Context, p models.BillingExplainerPayer) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	account, d := mapProviderCostsMap(
		ctx, p.Account,
		datasource_billing_explainer.AccountValue{}.Type(ctx),
		datasource_billing_explainer.AccountValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewAccountValue,
	)
	diags.Append(d...)

	service, d := mapProviderCostsMap(
		ctx, p.Service,
		datasource_billing_explainer.ServiceValue{}.Type(ctx),
		datasource_billing_explainer.ServiceValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewServiceValue,
	)
	diags.Append(d...)

	summary, d := mapSummary(ctx, p.Summary)
	diags.Append(d...)

	return map[string]attr.Value{
		"account":       account,
		"friendly_name": types.StringValue(p.FriendlyName),
		"payer_id":      types.StringValue(p.PayerId),
		"service":       service,
		"summary":       summary,
	}, diags
}

// mapProviderCostsMap builds the outer `account`/`service` map (keyed by
// service/account identifier, valued by a per-provider cost breakdown).
// AccountValue and ServiceValue are distinct generated types sharing the
// identical {aws, doit} shape, so this is generic over which one to build.
func mapProviderCostsMap[V attr.Value](ctx context.Context, costs models.BillingExplainerCostDifferences, elemType attr.Type, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.Map, diag.Diagnostics) {
	return buildObjectMap(
		elemType, attrTypes, costs,
		func(pc models.BillingExplainerProviderCosts) (map[string]attr.Value, diag.Diagnostics) {
			return providerCostsAttrs(ctx, pc)
		},
		newFn,
	)
}

// providerCostsAttrs builds the {aws, doit} pair shared identically by both
// AccountValue and ServiceValue - both wrap their aws/doit sub-maps with the
// same generated AwsValue/DoitValue element types.
func providerCostsAttrs(ctx context.Context, pc models.BillingExplainerProviderCosts) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	aws, d := mapCostDetailMap(
		pc.Aws,
		datasource_billing_explainer.AwsValue{}.Type(ctx),
		datasource_billing_explainer.AwsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewAwsValue,
	)
	diags.Append(d...)

	doit, d := mapCostDetailMap(
		pc.Doit,
		datasource_billing_explainer.DoitValue{}.Type(ctx),
		datasource_billing_explainer.DoitValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewDoitValue,
	)
	diags.Append(d...)

	return map[string]attr.Value{"aws": aws, "doit": doit}, diags
}

// mapCostDetailMap builds a cost-type-keyed map of money values (the leaf of
// the account/service double-map nesting).
func mapCostDetailMap[V attr.Value](detail models.BillingExplainerCostDetail, elemType attr.Type, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.Map, diag.Diagnostics) {
	return buildObjectMap(
		elemType, attrTypes, detail,
		func(m models.BillingExplainerMoney) (map[string]attr.Value, diag.Diagnostics) {
			return moneyAttrs(m.Amount, m.Currency), nil
		},
		newFn,
	)
}

func mapSummary(ctx context.Context, s models.BillingExplainerSummary) (datasource_billing_explainer.SummaryValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	aws, d := mapServiceSummary(
		ctx, s.Aws,
		datasource_billing_explainer.SummaryAwsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewSummaryAwsValue,
	)
	diags.Append(d...)

	doit, d := mapServiceSummary(
		ctx, s.Doit,
		datasource_billing_explainer.SummaryDoitValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewSummaryDoitValue,
	)
	diags.Append(d...)

	awsWithoutDoit, d := mapServiceSummary(
		ctx, s.AwsWithoutDoit,
		datasource_billing_explainer.AwsWithoutDoitValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewAwsWithoutDoitValue,
	)
	diags.Append(d...)

	val, d := datasource_billing_explainer.NewSummaryValue(
		datasource_billing_explainer.SummaryValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"aws":              aws,
			"aws_without_doit": awsWithoutDoit,
			"doit":             doit,
		},
	)
	diags.Append(d...)
	return val, diags
}

// mapServiceSummary builds the {credits, discounts, other_charges, refunds,
// savings, service_charges, support_charges, tax, total} shape shared
// identically by SummaryAwsValue, SummaryDoitValue and AwsWithoutDoitValue -
// three distinct generated types (path-qualified by the code generator since
// "aws"/"doit" also name the unrelated Money-map leaf types elsewhere in this
// schema), so this is generic over which constructor to use.
func mapServiceSummary[V attr.Value](ctx context.Context, s models.BillingExplainerServiceSummary, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (V, diag.Diagnostics) {
	var diags diag.Diagnostics

	credits, d := mapCostLineItems(
		ctx, s.Credits,
		datasource_billing_explainer.CreditsValue{}.Type(ctx),
		datasource_billing_explainer.CreditsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewCreditsValue,
	)
	diags.Append(d...)

	discounts, d := mapCostLineItems(
		ctx, s.Discounts,
		datasource_billing_explainer.DiscountsValue{}.Type(ctx),
		datasource_billing_explainer.DiscountsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewDiscountsValue,
	)
	diags.Append(d...)

	otherCharges, d := mapCostLineItems(
		ctx, s.OtherCharges,
		datasource_billing_explainer.OtherChargesValue{}.Type(ctx),
		datasource_billing_explainer.OtherChargesValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewOtherChargesValue,
	)
	diags.Append(d...)

	refunds, d := mapCostLineItems(
		ctx, s.Refunds,
		datasource_billing_explainer.RefundsValue{}.Type(ctx),
		datasource_billing_explainer.RefundsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewRefundsValue,
	)
	diags.Append(d...)

	savings, d := mapCostLineItems(
		ctx, s.Savings,
		datasource_billing_explainer.SavingsValue{}.Type(ctx),
		datasource_billing_explainer.SavingsValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewSavingsValue,
	)
	diags.Append(d...)

	serviceCharges, d := mapCostLineItems(
		ctx, s.ServiceCharges,
		datasource_billing_explainer.ServiceChargesValue{}.Type(ctx),
		datasource_billing_explainer.ServiceChargesValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewServiceChargesValue,
	)
	diags.Append(d...)

	tax, d := mapCostLineItems(
		ctx, s.Tax,
		datasource_billing_explainer.TaxValue{}.Type(ctx),
		datasource_billing_explainer.TaxValue{}.AttributeTypes(ctx),
		datasource_billing_explainer.NewTaxValue,
	)
	diags.Append(d...)

	supportCharges, d := mapSupportCharges(ctx, s.SupportCharges)
	diags.Append(d...)

	total, d := datasource_billing_explainer.NewTotalValue(
		datasource_billing_explainer.TotalValue{}.AttributeTypes(ctx),
		moneyAttrs(s.Total.Amount, s.Total.Currency),
	)
	diags.Append(d...)

	val, d := newFn(attrTypes, map[string]attr.Value{
		"credits":         credits,
		"discounts":       discounts,
		"other_charges":   otherCharges,
		"refunds":         refunds,
		"savings":         savings,
		"service_charges": serviceCharges,
		"support_charges": supportCharges,
		"tax":             tax,
		"total":           total,
	})
	diags.Append(d...)
	return val, diags
}

// mapCostLineItems maps a []BillingExplainerCostLineItem into a list of one
// of the seven generated per-field types (CreditsValue, DiscountsValue, ...)
// that all share the {cost, cost_type} shape.
func mapCostLineItems[V attr.Value](ctx context.Context, items []models.BillingExplainerCostLineItem, elemType attr.Type, attrTypes map[string]attr.Type, newFn func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (types.List, diag.Diagnostics) {
	return buildObjectList(elemType, attrTypes, len(items),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			cost, d := datasource_billing_explainer.NewCostValue(
				datasource_billing_explainer.CostValue{}.AttributeTypes(ctx),
				moneyAttrs(items[i].Cost.Amount, items[i].Cost.Currency),
			)
			return map[string]attr.Value{
				"cost":      cost,
				"cost_type": types.StringValue(items[i].CostType),
			}, d
		},
		newFn,
	)
}

func mapSupportCharges(ctx context.Context, sc models.BillingExplainerSupportCharges) (datasource_billing_explainer.SupportChargesValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	cost, d := datasource_billing_explainer.NewCostValue(
		datasource_billing_explainer.CostValue{}.AttributeTypes(ctx),
		moneyAttrs(sc.Cost.Amount, sc.Cost.Currency),
	)
	diags.Append(d...)

	details, d := mapSupportDetails(ctx, sc.Details)
	diags.Append(d...)

	val, d := datasource_billing_explainer.NewSupportChargesValue(
		datasource_billing_explainer.SupportChargesValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"cost": cost, "details": details},
	)
	diags.Append(d...)
	return val, diags
}

func mapSupportDetails(ctx context.Context, details []models.BillingExplainerSupportDetail) (types.List, diag.Diagnostics) {
	return buildObjectList(
		datasource_billing_explainer.DetailsValue{}.Type(ctx),
		datasource_billing_explainer.DetailsValue{}.AttributeTypes(ctx),
		len(details),
		func(i int) (map[string]attr.Value, diag.Diagnostics) {
			var diags diag.Diagnostics
			detail := details[i]

			baseCost, d := datasource_billing_explainer.NewBaseCostValue(
				datasource_billing_explainer.BaseCostValue{}.AttributeTypes(ctx),
				moneyAttrs(detail.BaseCost.Amount, detail.BaseCost.Currency),
			)
			diags.Append(d...)

			cost, d := datasource_billing_explainer.NewCostValue(
				datasource_billing_explainer.CostValue{}.AttributeTypes(ctx),
				moneyAttrs(detail.Cost.Amount, detail.Cost.Currency),
			)
			diags.Append(d...)

			return map[string]attr.Value{
				"base_cost":           baseCost,
				"cost":                cost,
				"description":         types.StringValue(detail.Description),
				"project_id":          types.StringValue(detail.ProjectId),
				"service_description": types.StringValue(detail.ServiceDescription),
			}, diags
		},
		datasource_billing_explainer.NewDetailsValue,
	)
}
