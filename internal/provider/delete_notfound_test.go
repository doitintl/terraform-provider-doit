package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
					AttributeTypes: getAttributeTypes(schemaResp.Schema.Attributes, ctx),
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
					if contains(d.Detail(), tt.errorContains) || contains(d.Summary(), tt.errorContains) {
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
					AttributeTypes: getAttributeTypes(schemaResp.Schema.Attributes, ctx),
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
					AttributeTypes: getAttributeTypes(schemaResp.Schema.Attributes, ctx),
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to get attribute types from schema attributes
func getAttributeTypes(attrs map[string]schema.Attribute, ctx context.Context) map[string]tftypes.Type {
	result := make(map[string]tftypes.Type)
	for name, attr := range attrs {
		result[name] = attr.GetType().TerraformType(ctx)
	}
	return result
}

// Test that the 404 detection logic works correctly for error messages
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
			result := findSubstring(tt.errMsg, "404")
			if result != tt.expected {
				t.Errorf("is404Error(%q) = %v, expected %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

// Test with RetryClient to verify 404 responses pass through for proper handling
// (not converted to errors like other 4xx codes)
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

// Verify that the Delete function behavior is correct when using RetryClient
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
					AttributeTypes: getAttributeTypes(schemaResp.Schema.Attributes, ctx),
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
