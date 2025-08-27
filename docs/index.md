# blobleas Provider

The `blobleas` provider is used to manage Azure Blob Storage leases. It creates blobs with leases and manages their lifecycle.

## Example Usage

```hcl
provider "blobleas" {
  # Uses DefaultAzureCredential for authentication
}

resource "blobleas_blob_lease" "lock_file" {
  storage_account = "mystorageaccount" 
  container_name  = "locks"
  blob_name      = "application.lock"
  content        = "managed by terraform"
}
```
