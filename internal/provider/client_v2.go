package provider

import (
	"fmt"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	modelsv2 "github.com/doitintl/terraform-provider-doit/internal/provider/models/v2"
)

// newV2Client builds the client for the v2 API (api-specs, package modelsv2)
// from the provider's client. Both APIs are served from the same host, so the
// v2 client reuses the host, the retrying HTTP client and the request editors
// (authentication, User-Agent and X-Tenant-Id) of the provider's client.
func newV2Client(client *models.ClientWithResponses) (*modelsv2.ClientWithResponses, error) {
	base, ok := client.ClientInterface.(*models.Client)
	if !ok {
		return nil, fmt.Errorf("expected *models.Client, got: %T", client.ClientInterface)
	}

	opts := []modelsv2.ClientOption{modelsv2.WithHTTPClient(base.Client)}
	for _, editor := range base.RequestEditors {
		opts = append(opts, modelsv2.WithRequestEditorFn(modelsv2.RequestEditorFn(editor)))
	}

	return modelsv2.NewClientWithResponses(base.Server, opts...)
}
