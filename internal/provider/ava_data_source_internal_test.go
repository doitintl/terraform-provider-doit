package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestAvaDataSource_Read_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "success - answer present",
			responseBody: `{"answer": "You are using AWS and GCP."}`,
			expectError:  false,
		},
		{
			name:          "generation failure - error present",
			responseBody:  `{"error": "generation failed: internal timeout"}`,
			expectError:   true,
			errorContains: "generation failed: internal timeout",
		},
		{
			name:          "empty body - JSON parse error",
			responseBody:  ``,
			expectError:   true,
			errorContains: "Unable to send question to Ava",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			client, err := models.NewClientWithResponses(server.URL)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ds := &avaDataSource{client: client}
			ctx := context.Background()

			schemaResp := &datasource.SchemaResponse{}
			ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Failed to get schema: %v", schemaResp.Diagnostics)
			}

			configValues := map[string]tftypes.Value{
				"question": tftypes.NewValue(tftypes.String, "What cloud providers am I using?"),
			}
			for attrName, attr := range schemaResp.Schema.Attributes {
				if attrName == "question" {
					continue
				}
				configValues[attrName] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
			}

			attrTypes := make(map[string]tftypes.Type)
			for name, attr := range schemaResp.Schema.Attributes {
				attrTypes[name] = attr.GetType().TerraformType(ctx)
			}
			configType := tftypes.Object{
				AttributeTypes: attrTypes,
			}
			configVal := tftypes.NewValue(configType, configValues)
			config := tfsdk.Config{
				Schema: schemaResp.Schema,
				Raw:    configVal,
			}

			readReq := datasource.ReadRequest{
				Config: config,
			}
			readResp := &datasource.ReadResponse{
				State: tfsdk.State{
					Schema: schemaResp.Schema,
				},
			}

			ds.Read(ctx, readReq, readResp)

			hasError := readResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				t.Errorf("Read() hasError = %v, expectError %v; diagnostics: %v",
					hasError, tt.expectError, readResp.Diagnostics)
			}

			if tt.expectError && tt.errorContains != "" {
				found := false
				for _, d := range readResp.Diagnostics {
					if strings.Contains(d.Detail(), tt.errorContains) || strings.Contains(d.Summary(), tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, readResp.Diagnostics)
				}
			}
		})
	}
}
