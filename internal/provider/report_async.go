package provider

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const asyncPollInterval = time.Second
const asyncCancelTimeout = 30 * time.Second

func asyncReportAttribute() schema.BoolAttribute {
	description := "Execute asynchronously by submitting a job and polling for results. Defaults to false. The read timeout covers submission, polling and results. On interruption or timeout, the provider attempts to cancel the known operation. Identical concurrent queries may share an operation, so cancellation can affect another reader using the same account. The async API does not currently expose details.valueAliases."
	return schema.BoolAttribute{Optional: true, Description: description, MarkdownDescription: description}
}

func runAsyncReport(ctx context.Context, client *models.ClientWithResponses, config *models.ExternalConfig, reportID string, params *models.GetReportParams) (result *models.GetAsyncOperationResults200Response, diags diag.Diagnostics) {
	key := rand.Text()
	var operation *models.AsyncOperationResponse
	if config != nil {
		resp, err := client.AsyncRunInlineWithResponse(ctx, &models.AsyncRunInlineParams{IdempotencyKey: key}, models.AsyncRunInlineJSONRequestBody{Config: config})
		if err != nil {
			diags.AddError("Error Submitting Async Report", err.Error())
			return
		}
		if resp.StatusCode() != http.StatusAccepted || resp.JSON202 == nil {
			diags.AddError("Error Submitting Async Report", fmt.Sprintf("status: %d, body: %s", resp.StatusCode(), resp.Body))
			return
		}
		operation = resp.JSON202
	} else {
		resp, err := client.AsyncRunReportByIdWithResponse(ctx, reportID, &models.AsyncRunReportByIdParams{IdempotencyKey: key, TimeRange: params.TimeRange, StartDate: params.StartDate, EndDate: params.EndDate})
		if err != nil {
			diags.AddError("Error Submitting Async Report", err.Error())
			return
		}
		if resp.StatusCode() != http.StatusAccepted || resp.JSON202 == nil {
			diags.AddError("Error Submitting Async Report", fmt.Sprintf("status: %d, body: %s", resp.StatusCode(), resp.Body))
			return
		}
		operation = resp.JSON202
	}
	if operation.OperationId == nil || *operation.OperationId == "" {
		diags.AddError("Invalid Async Report Response", "The accepted response did not contain an operation ID.")
		return
	}
	operationID := *operation.OperationId
	terminal := false
	defer func() {
		if ctx.Err() == nil || terminal {
			return
		}
		// Cleanup needs its own bounded context because the read context is already canceled.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), asyncCancelTimeout)
		defer cancel()
		resp, err := client.CancelAsyncOperationWithResponse(cleanupCtx, operationID, &models.CancelAsyncOperationParams{IdempotencyKey: rand.Text()})
		if err != nil {
			diags.AddWarning("Could Not Cancel Async Report", fmt.Sprintf("Operation %s may still be running: %v", operationID, err))
			return
		}
		if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNotFound {
			diags.AddWarning("Could Not Cancel Async Report", fmt.Sprintf("Operation %s may still be running, status: %d, body: %s", operationID, resp.StatusCode(), resp.Body))
		}
	}()
	for {
		resp, err := client.GetAsyncOperationWithResponse(ctx, operationID)
		if err != nil {
			diags.AddError("Error Polling Async Report", fmt.Sprintf("Operation %s: %v", operationID, err))
			return
		}
		if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
			diags.AddError("Error Polling Async Report", fmt.Sprintf("Operation %s, status: %d, body: %s", operationID, resp.StatusCode(), resp.Body))
			return
		}
		if resp.JSON200.OperationId != operationID {
			diags.AddError("Invalid Async Report Response", "The poll response did not match the requested operation ID.")
			return
		}
		switch resp.JSON200.Status {
		case models.AsyncOperationPollResponseStatusSucceeded:
			terminal = true
			resultResp, err := client.GetAsyncOperationResultsWithResponse(ctx, operationID)
			if err != nil {
				diags.AddError("Error Reading Async Report Results", fmt.Sprintf("Operation %s: %v", operationID, err))
				return
			}
			if resultResp.StatusCode() != http.StatusOK || resultResp.JSON200 == nil {
				diags.AddError("Error Reading Async Report Results", fmt.Sprintf("Operation %s, status: %d, body: %s", operationID, resultResp.StatusCode(), resultResp.Body))
				return
			}
			return resultResp.JSON200, diags
		case models.AsyncOperationPollResponseStatusFailed, models.AsyncOperationPollResponseStatusCanceled:
			terminal = true
			detail := fmt.Sprintf("Operation %s ended with status %s.", operationID, resp.JSON200.Status)
			if failure := resp.JSON200.Error; failure != nil {
				detail += fmt.Sprintf(" %s (code: %s, status: %d)", failure.Title, failure.Code, failure.Status)
				if failure.Detail != nil {
					detail += ": " + *failure.Detail
				}
			}
			diags.AddError("Async Report Did Not Succeed", detail)
			return
		case models.AsyncOperationPollResponseStatusPending, models.AsyncOperationPollResponseStatusRunning:
			delay := asyncPollInterval
			if seconds, err := strconv.ParseInt(resp.HTTPResponse.Header.Get("Retry-After"), 10, 64); err == nil && seconds > 0 {
				delay = time.Duration(min(seconds, int64(60))) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				diags.AddError("Async Report Interrupted", fmt.Sprintf("Operation %s: %v", operationID, ctx.Err()))
				return
			case <-timer.C:
			}
		default:
			diags.AddError("Invalid Async Report Response", fmt.Sprintf("Operation %s returned unknown status %q.", operationID, resp.JSON200.Status))
			return
		}
	}
}

func asyncReportOutputs(result *models.GetAsyncOperationResults200ResponseResult) (types.String, types.Bool, types.Int64, diag.Diagnostics) {
	if result == nil {
		return types.StringValue("{}"), types.BoolNull(), types.Int64Value(0), nil
	}
	var diags diag.Diagnostics
	body, err := json.Marshal(result)
	if err != nil {
		diags.AddError("Error Serializing Async Report Results", err.Error())
	}
	var rows int64
	if result.Rows != nil {
		rows = int64(len(*result.Rows))
	}
	return types.StringValue(string(body)), types.BoolPointerValue(result.CacheHit), types.Int64Value(rows), diags
}
