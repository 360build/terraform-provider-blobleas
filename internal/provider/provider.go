package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	blobclient "github.com/360-build/terraform-provider-blobleas/internal/provider/blobclient"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &blobLeaseProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &blobLeaseProvider{
			version: version,
		}
	}
}

// blobLeaseProvider is the provider implementation.
type blobLeaseProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *blobLeaseProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "blobleas"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *blobLeaseProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Azure Blob Storage Lease provider for managing blob leases across Azure Storage accounts",
		// No configuration attributes needed - uses DefaultAzureCredential
		Attributes: map[string]schema.Attribute{},
	}
}

// Configure prepares an Azure Blob Storage lease client for data sources and resources.
func (p *blobLeaseProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Create the Azure Blob Storage lease client
	client, err := blobclient.NewAzureBlobLeaseClient()
	if err != nil {
		resp.Diagnostics.AddError("Failed to initialize Azure Blob Storage lease client", err.Error())
		return
	}

	// Store the client in the context
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *blobLeaseProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *blobLeaseProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBlobLeaseResource,
	}
}
