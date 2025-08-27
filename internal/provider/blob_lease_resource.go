package provider

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	blobclient "github.com/360-build/terraform-provider-blobleas/internal/provider/blobclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BlobLeaseResource{}
var _ resource.ResourceWithImportState = &BlobLeaseResource{}

// Custom plan modifier to check lease state and trigger updates when needed
type leaseStatePlanModifier struct{}

func (m leaseStatePlanModifier) Description(ctx context.Context) string {
	return "Checks lease state and triggers update if lease needs renewal"
}

func (m leaseStatePlanModifier) MarkdownDescription(ctx context.Context) string {
	return "Checks lease state and triggers update if lease needs renewal"
}

func (m leaseStatePlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Don't modify during create
	if req.State.Raw.IsNull() {
		return
	}

	// Get current state value
	var stateValue types.String
	diags := req.State.GetAttribute(ctx, req.Path, &stateValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If lease state is not "leased", mark for update
	if !stateValue.IsNull() && !stateValue.IsUnknown() && stateValue.ValueString() != "leased" {
		resp.PlanValue = types.StringUnknown()
		resp.RequiresReplace = false
	}
}

func NewBlobLeaseResource() resource.Resource {
	return &BlobLeaseResource{}
}

// BlobLeaseResource defines the resource implementation.
type BlobLeaseResource struct {
	client *blobclient.AzureBlobLeaseClient
}

// BlobLeaseResourceModel describes the resource data model.
type BlobLeaseResourceModel struct {
	ID             types.String `tfsdk:"id"`
	StorageAccount types.String `tfsdk:"storage_account"`
	ContainerName  types.String `tfsdk:"container_name"`
	BlobName       types.String `tfsdk:"blob_name"`
	Content        types.String `tfsdk:"content"`
	LeaseID        types.String `tfsdk:"lease_id"`
	BlobURL        types.String `tfsdk:"blob_url"`
	ETag           types.String `tfsdk:"etag"`
	LeaseState     types.String `tfsdk:"lease_state"`
}

func (r *BlobLeaseResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_blob_lease"
}

func (r *BlobLeaseResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Azure Blob Storage lease resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"storage_account": schema.StringAttribute{
				MarkdownDescription: "The Azure Storage Account name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"container_name": schema.StringAttribute{
				MarkdownDescription: "The container name where the blob will be created",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"blob_name": schema.StringAttribute{
				MarkdownDescription: "The name of the blob to create and lease",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "The content to write to the blob",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"lease_id": schema.StringAttribute{
				MarkdownDescription: "The lease ID for the blob",
				Computed:            true,
			},
			"blob_url": schema.StringAttribute{
				MarkdownDescription: "The URL of the blob",
				Computed:            true,
			},
			"etag": schema.StringAttribute{
				MarkdownDescription: "The ETag of the blob",
				Computed:            true,
			},
			"lease_state": schema.StringAttribute{
				MarkdownDescription: "The current lease state of the blob",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					leaseStatePlanModifier{},
				},
			},
		},
	}
}

func (r *BlobLeaseResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*blobclient.AzureBlobLeaseClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *blobclient.AzureBlobLeaseClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *BlobLeaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BlobLeaseResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Set default content if not provided
	content := "managed by terraform-provider-blobleas"
	if !data.Content.IsNull() && !data.Content.IsUnknown() {
		content = data.Content.ValueString()
	}

	// Generate a unique lease ID (must be a valid UUID for Azure)
	leaseID := uuid.New().String()

	// Create blob with lease
	config := blobclient.BlobLeaseConfig{
		StorageAccount: data.StorageAccount.ValueString(),
		ContainerName:  data.ContainerName.ValueString(),
		BlobName:       data.BlobName.ValueString(),
		Content:        []byte(content),
		LeaseID:        leaseID,
	}

	result, err := r.client.CreateBlobWithLease(ctx, config)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create blob with lease, got error: %s", err))
		return
	}

	// Set computed attributes
	data.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", config.StorageAccount, config.ContainerName, config.BlobName))
	data.LeaseID = types.StringValue(result.LeaseID)
	data.BlobURL = types.StringValue(result.BlobURL)
	data.ETag = types.StringValue(result.ETag)
	data.LeaseState = types.StringValue(result.LeaseState)
	if data.Content.IsNull() || data.Content.IsUnknown() {
		data.Content = types.StringValue(content)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BlobLeaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BlobLeaseResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Check if blob still exists
	exists, err := r.client.BlobExists(ctx, data.StorageAccount.ValueString(), data.ContainerName.ValueString(), data.BlobName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to check blob existence, got error: %s", err))
		return
	}

	if !exists {
		// Blob was deleted outside of Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	// Get current lease state
	leaseResult, err := r.client.GetBlobLeaseState(ctx, data.StorageAccount.ValueString(), data.ContainerName.ValueString(), data.BlobName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read blob lease state, got error: %s", err))
		return
	}

	// Update computed attributes
	data.ETag = types.StringValue(leaseResult.ETag)
	data.LeaseState = types.StringValue(leaseResult.LeaseState)

	// Don't automatically renew lease during read - let Terraform detect drift
	// The Update function will handle lease renewal during apply

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BlobLeaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BlobLeaseResourceModel
	var state BlobLeaseResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read current state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check current lease state
	leaseResult, err := r.client.GetBlobLeaseState(ctx, data.StorageAccount.ValueString(), data.ContainerName.ValueString(), data.BlobName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read blob lease state, got error: %s", err))
		return
	}

	// If lease is not active, try to renew or acquire a new lease
	if leaseResult.LeaseState != "leased" {
		config := blobclient.BlobLeaseConfig{
			StorageAccount: data.StorageAccount.ValueString(),
			ContainerName:  data.ContainerName.ValueString(),
			BlobName:       data.BlobName.ValueString(),
			LeaseID:        state.LeaseID.ValueString(),
		}

		// Try to renew existing lease first
		result, err := r.client.RenewBlobLease(ctx, config)
		if err != nil {
			// If renewal fails, try to acquire a new lease
			config.LeaseID = uuid.New().String()
			content := "managed by terraform-provider-blobleas"
			if !data.Content.IsNull() && !data.Content.IsUnknown() {
				content = data.Content.ValueString()
			}
			config.Content = []byte(content)
			
			result, err = r.client.CreateBlobWithLease(ctx, config)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to renew or acquire blob lease, got error: %s", err))
				return
			}
		}

		// Update computed attributes
		data.LeaseID = types.StringValue(result.LeaseID)
		data.ETag = types.StringValue(result.ETag)
		data.LeaseState = types.StringValue(result.LeaseState)
		data.BlobURL = types.StringValue(result.BlobURL)
	} else {
		// Lease is still active, just update metadata
		data.ETag = types.StringValue(leaseResult.ETag)
		data.LeaseState = types.StringValue(leaseResult.LeaseState)
		data.BlobURL = types.StringValue(leaseResult.BlobURL)
		data.LeaseID = state.LeaseID // Keep existing lease ID
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BlobLeaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BlobLeaseResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Release lease and delete blob
	config := blobclient.BlobLeaseConfig{
		StorageAccount: data.StorageAccount.ValueString(),
		ContainerName:  data.ContainerName.ValueString(),
		BlobName:       data.BlobName.ValueString(),
		LeaseID:        data.LeaseID.ValueString(),
	}

	err := r.client.ReleaseBlobLease(ctx, config, true) // true = delete blob
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to release lease and delete blob, got error: %s", err))
		return
	}
}

func (r *BlobLeaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: storage_account/container_name/blob_name
	id := req.ID

	parts := []string{}
	// Simple split by '/'
	start := 0
	for i, char := range id {
		if char == '/' {
			if start < i {
				parts = append(parts, id[start:i])
			}
			start = i + 1
		}
	}
	if start < len(id) {
		parts = append(parts, id[start:])
	}

	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: storage_account/container_name/blob_name. Got: %s", req.ID),
		)
		return
	}

	storageAccount := parts[0]
	containerName := parts[1]
	blobName := parts[2]

	// Check if blob exists
	exists, err := r.client.BlobExists(ctx, storageAccount, containerName, blobName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to check blob existence during import, got error: %s", err))
		return
	}

	if !exists {
		resp.Diagnostics.AddError(
			"Resource Not Found",
			fmt.Sprintf("Blob %s does not exist in container %s of storage account %s", blobName, containerName, storageAccount),
		)
		return
	}

	// Get current lease state
	leaseResult, err := r.client.GetBlobLeaseState(ctx, storageAccount, containerName, blobName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read blob lease state during import, got error: %s", err))
		return
	}

	// Set the state
	var data BlobLeaseResourceModel
	data.ID = types.StringValue(req.ID)
	data.StorageAccount = types.StringValue(storageAccount)
	data.ContainerName = types.StringValue(containerName)
	data.BlobName = types.StringValue(blobName)
	data.Content = types.StringValue("") // Cannot read blob content during import
	data.BlobURL = types.StringValue(leaseResult.BlobURL)
	data.ETag = types.StringValue(leaseResult.ETag)
	data.LeaseState = types.StringValue(leaseResult.LeaseState)
	data.LeaseID = types.StringValue("") // Unknown lease ID during import

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}