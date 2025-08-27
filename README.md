# Terraform Provider blobleas

This provider enables Azure Blob Storage lease management operations with Terraform.

## Features

- **Azure Blob Lease Management**: Create and manage blob leases in Azure Storage accounts
- **Automatic Lease Renewal**: Handles lease renewal to prevent expiration
- **Azure Native**: Uses Azure SDK and DefaultAzureCredential for seamless Azure integration
- **Lease Monitoring**: Automatically re-acquires leases if they are released externally

## Key Benefits

- **Resource Locking**: Use blob leases to implement distributed locking mechanisms
- **State Management**: Manage application state with lease-protected blobs
- **Azure Integration**: Leverages Azure Storage's built-in lease capabilities
- **Infrastructure as Code**: Manage blob leases through Terraform

## Usage

```hcl
provider "blobleas" {
  # Uses DefaultAzureCredential for authentication
}

resource "blobleas_blob_lease" "lock_file" {
  storage_account = "mystorageaccount"
  container_name  = "locks"
  blob_name      = "application.lock"
  content        = "managed by terraform-provider-blobleas"
}
```

## Resource: blobleas_blob_lease

### Arguments

- `storage_account` (Required) - The Azure Storage Account name
- `container_name` (Required) - The container name where the blob will be created  
- `blob_name` (Required) - The name of the blob to create and lease
- `content` (Optional) - The content to write to the blob (defaults to "managed by terraform-provider-blobleas")

### Attributes

- `id` - Resource identifier in format `storage_account/container_name/blob_name`
- `lease_id` - The lease ID for the blob
- `blob_url` - The URL of the blob
- `etag` - The ETag of the blob
- `lease_state` - The current lease state of the blob

## Authentication

The provider uses Azure's DefaultAzureCredential, which supports:
- Environment variables (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID)
- Managed Identity
- Azure CLI authentication
- Visual Studio authentication
