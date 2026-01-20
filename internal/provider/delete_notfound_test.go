package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestBudgetResourceDelete_NotFound tests that deleting a budget that returns 404
// is treated as a successful delete (resource already gone).
func TestBudgetResourceDelete_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "200 OK - successful delete",
			statusCode:   http.StatusOK,
			responseBody: "",
			expectError:  false,
		},
		{
			name:         "204 No Content - successful delete",
			statusCode:   http.StatusNoContent,
			responseBody: "",
			expectError:  false,
		},
		{
			name:         "404 Not Found - resource already deleted",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message": "Budget not found"}`,
			expectError:  false, // 404 should be treated as success
		},
		{
			name:          "500 Internal Server Error - should fail",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"message": "Internal server error"}`,
			expectError:   true,
			errorContains: "500",
		},
		{
			name:          "403 Forbidden - should fail",
			statusCode:    http.StatusForbidden,
			responseBody:  `{"message": "Access denied"}`,
			expectError:   true,
			errorContains: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server that returns the specified status code
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create client pointing to mock server
			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create the resource
			r := &budgetResource{client: client}

			// Create a mock state with a budget ID
			ctx := context.Background()

			// Build the state schema
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			// Create state with minimal required attributes
			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-budget-id"),
			}
			// Add all other attributes as null/unknown
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			// Call Delete
			deleteReq := resource.DeleteRequest{
				State: state,
			}
			deleteResp := &resource.DeleteResponse{}
			r.Delete(ctx, deleteReq, deleteResp)

			// Check results
			hasError := deleteResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Delete() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, deleteResp.Diagnostics)
			}

			if tt.expectError && tt.errorContains != "" {
				found := false
				for _, d := range deleteResp.Diagnostics {
					if strings.Contains(d.Detail(), tt.errorContains) || strings.Contains(d.Summary(), tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, deleteResp.Diagnostics)
				}
			}
		})
	}
}

// TestAllocationResourceDelete_NotFound tests that deleting an allocation that returns 404
// is treated as a successful delete (resource already gone).
func TestAllocationResourceDelete_NotFound(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "200 OK - successful delete",
			statusCode:   http.StatusOK,
			responseBody: "",
			expectError:  false,
		},
		{
			name:         "404 Not Found - resource already deleted",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message": "Allocation not found"}`,
			expectError:  false, // 404 should be treated as success
		},
		{
			name:         "500 Internal Server Error - should fail",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"message": "Internal server error"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create client pointing to mock server
			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create the resource
			r := &allocationResource{client: client}

			// Build the state schema
			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			// Create state with minimal required attributes
			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-allocation-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			// Call Delete
			deleteReq := resource.DeleteRequest{
				State: state,
			}
			deleteResp := &resource.DeleteResponse{}
			r.Delete(ctx, deleteReq, deleteResp)

			// Check results
			hasError := deleteResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Delete() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, deleteResp.Diagnostics)
			}
		})
	}
}

// TestReportResourceDelete_NotFound tests that deleting a report that returns 404
// is treated as a successful delete (resource already gone).
func TestReportResourceDelete_NotFound(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "200 OK - successful delete",
			statusCode:   http.StatusOK,
			responseBody: "",
			expectError:  false,
		},
		{
			name:         "404 Not Found - resource already deleted",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message": "Report not found"}`,
			expectError:  false, // 404 should be treated as success
		},
		{
			name:         "500 Internal Server Error - should fail",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"message": "Internal server error"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create client pointing to mock server
			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create the resource
			r := &reportResource{client: client}

			// Build the state schema
			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			// Create state with minimal required attributes
			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-report-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			// Call Delete
			deleteReq := resource.DeleteRequest{
				State: state,
			}
			deleteResp := &resource.DeleteResponse{}
			r.Delete(ctx, deleteReq, deleteResp)

			// Check results
			hasError := deleteResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Delete() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, deleteResp.Diagnostics)
			}
		})
	}
}

// getAttributeTypes returns attribute types from schema attributes.
func getAttributeTypes(ctx context.Context, attrs map[string]schema.Attribute) map[string]tftypes.Type {
	result := make(map[string]tftypes.Type)
	for name, attr := range attrs {
		result[name] = attr.GetType().TerraformType(ctx)
	}
	return result
}

// TestIs404Error tests that the 404 detection logic works correctly for error messages.
func TestIs404Error(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "Contains 404 status code",
			errMsg:   "non-retryable error: 404, body: {\"message\": \"not found\"}",
			expected: true,
		},
		{
			name:     "Contains 404 in message",
			errMsg:   "resource not found: 404",
			expected: true,
		},
		{
			name:     "Does not contain 404",
			errMsg:   "non-retryable error: 500, body: {\"message\": \"internal error\"}",
			expected: false,
		},
		{
			name:     "Contains 403 not 404",
			errMsg:   "non-retryable error: 403, body: {\"message\": \"forbidden\"}",
			expected: false,
		},
		{
			name:     "Empty error message",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the check used in Delete functions
			result := strings.Contains(tt.errMsg, "404")
			if result != tt.expected {
				t.Errorf("is404Error(%q) = %v, expected %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

// TestBudgetDelete_WithRetryClient_404 tests that 404 responses pass through for proper handling
// (not converted to errors like other 4xx codes).
func TestBudgetDelete_WithRetryClient_404(t *testing.T) {
	// Create a mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Budget not found"}`))
	}))
	defer server.Close()

	// Create client with RetryClient (like the real provider does)
	retryClient := &RetryClient{
		client: &http.Client{},
	}

	client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(retryClient))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Try to delete - 404 should pass through as a response (not error)
	ctx := context.Background()
	resp, err := client.DeleteBudgetWithResponse(ctx, "test-id")

	// With the new RetryClient behavior, 404 passes through as a response
	if err != nil {
		t.Errorf("Expected no error for 404 (should pass through), got: %v", err)
	}

	// Verify we got a 404 response that can be inspected
	if resp != nil && resp.StatusCode() != http.StatusNotFound {
		t.Errorf("Expected status 404, got: %d", resp.StatusCode())
	}
}

// TestBudgetResourceRead_NotFound tests that reading a budget that returns 404
// properly removes the resource from state (externally deleted resource).
func TestBudgetResourceRead_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectRemoved bool // true if resource should be removed from state
		expectError   bool
	}{
		{
			name:          "200 OK - resource exists",
			statusCode:    http.StatusOK,
			responseBody:  `{"id": "test-budget-id", "name": "Test Budget", "amount": 1000, "type": "fixed", "currency": "USD", "startPeriod": 1704067200000, "timeInterval": "month", "scopes": []}`,
			expectRemoved: false,
			expectError:   false,
		},
		{
			name:          "404 Not Found - resource externally deleted",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"message": "Budget not found"}`,
			expectRemoved: true, // 404 should remove resource from state
			expectError:   false,
		},
		{
			name:          "500 Internal Server Error - should fail",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"message": "Internal server error"}`,
			expectRemoved: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create client pointing to mock server
			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create the resource
			r := &budgetResource{client: client}

			// Build the state schema
			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			// Create state with minimal required attributes
			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-budget-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			// Call Read
			readReq := resource.ReadRequest{
				State: state,
			}
			readResp := &resource.ReadResponse{
				State: state,
			}
			r.Read(ctx, readReq, readResp)

			// Check results
			hasError := readResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Read() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, readResp.Diagnostics)
			}

			// For 404, check that state.Raw is null (resource removed)
			if tt.expectRemoved && !hasError {
				if !readResp.State.Raw.IsNull() {
					// Get the ID from state to check if it's null
					var resultState budgetResourceModel
					diags := readResp.State.Get(ctx, &resultState)
					if diags.HasError() {
						// State might be completely null which is expected
						return
					}
					if !resultState.Id.IsNull() {
						t.Errorf("Expected resource to be removed from state (ID should be null), got ID: %s", resultState.Id.ValueString())
					}
				}
			}
		})
	}
}

// TestAllocationResourceRead_NotFound tests that reading an allocation that returns 404
// properly removes the resource from state.
func TestAllocationResourceRead_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectRemoved bool
		expectError   bool
	}{
		{
			name:          "200 OK - resource exists",
			statusCode:    http.StatusOK,
			responseBody:  `{"id": "test-allocation-id", "name": "Test Allocation", "rules": []}`,
			expectRemoved: false,
			expectError:   false,
		},
		{
			name:          "404 Not Found - resource externally deleted",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"message": "Allocation not found"}`,
			expectRemoved: true,
			expectError:   false,
		},
		{
			name:          "500 Internal Server Error - should fail",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"message": "Internal server error"}`,
			expectRemoved: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			r := &allocationResource{client: client}

			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-allocation-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			readReq := resource.ReadRequest{State: state}
			readResp := &resource.ReadResponse{State: state}
			r.Read(ctx, readReq, readResp)

			hasError := readResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Read() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, readResp.Diagnostics)
			}

			if tt.expectRemoved && !hasError {
				if !readResp.State.Raw.IsNull() {
					var resultState allocationResourceModel
					diags := readResp.State.Get(ctx, &resultState)
					if diags.HasError() {
						return
					}
					if !resultState.Id.IsNull() {
						t.Errorf("Expected resource to be removed from state (ID should be null), got ID: %s", resultState.Id.ValueString())
					}
				}
			}
		})
	}
}

// TestReportResourceRead_NotFound tests that reading a report that returns 404
// properly removes the resource from state.
func TestReportResourceRead_NotFound(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectRemoved bool
		expectError   bool
	}{
		{
			name:          "200 OK - resource exists",
			statusCode:    http.StatusOK,
			responseBody:  `{"id": "test-report-id", "name": "Test Report", "config": {"advancedAnalysis": {"enabled": false}}}`,
			expectRemoved: false,
			expectError:   false,
		},
		{
			name:          "404 Not Found - resource externally deleted",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"message": "Report not found"}`,
			expectRemoved: true,
			expectError:   false,
		},
		{
			name:          "500 Internal Server Error - should fail",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"message": "Internal server error"}`,
			expectRemoved: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			r := &reportResource{client: client}

			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-report-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			readReq := resource.ReadRequest{State: state}
			readResp := &resource.ReadResponse{State: state}
			r.Read(ctx, readReq, readResp)

			hasError := readResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Read() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, readResp.Diagnostics)
			}

			if tt.expectRemoved && !hasError {
				if !readResp.State.Raw.IsNull() {
					var resultState reportResourceModel
					diags := readResp.State.Get(ctx, &resultState)
					if diags.HasError() {
						return
					}
					if !resultState.Id.IsNull() {
						t.Errorf("Expected resource to be removed from state (ID should be null), got ID: %s", resultState.Id.ValueString())
					}
				}
			}
		})
	}
}

// TestBudgetResourceDelete_WithRetryClient_Integration verifies that the Delete function behavior is correct when using RetryClient.
func TestBudgetResourceDelete_WithRetryClient_Integration(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "404 via RetryClient - should be treated as success",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message": "not found"}`,
			expectError:  false,
		},
		{
			name:         "500 via RetryClient - should fail",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"message": "internal error"}`,
			expectError:  true,
		},
		{
			name:         "200 OK - success",
			statusCode:   http.StatusOK,
			responseBody: "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.responseBody != "" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create client WITH RetryClient (as in production)
			retryClient := &RetryClient{
				client: &http.Client{},
			}

			client, err := models.NewClientWithResponses(server.URL, models.WithHTTPClient(retryClient))
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Create the resource
			r := &budgetResource{client: client}

			// Build the state schema
			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

			// Create state
			stateValues := map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "test-budget-id"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "id" {
					continue
				}
				stateValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			stateValue := tftypes.NewValue(
				tftypes.Object{
					AttributeTypes: getAttributeTypes(ctx, schemaResp.Schema.Attributes),
				},
				stateValues,
			)

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    stateValue,
			}

			// Call Delete
			deleteReq := resource.DeleteRequest{State: state}
			deleteResp := &resource.DeleteResponse{}
			r.Delete(ctx, deleteReq, deleteResp)

			// Check results
			hasError := deleteResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				var diagDetails string
				for _, d := range deleteResp.Diagnostics {
					diagDetails += fmt.Sprintf("\n  - %s: %s", d.Summary(), d.Detail())
				}
				t.Errorf("Delete() hasError = %v, expectError %v; diagnostics:%s",
					hasError, tt.expectError, diagDetails)
			}
		})
	}
}
