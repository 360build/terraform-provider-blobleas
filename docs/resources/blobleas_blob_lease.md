# blobleas_blob_lease Resource

Manages an Azure Blob Storage lease. Creates a blob in the specified container and acquires a lease on it. When the resource is destroyed, the lease is released and the blob is deleted.

## Example Usage

```hcl
resource "blobleas_blob_lease" "example" {
  storage_account = "mystorageaccount"
  container_name  = "mycontainer"
  blob_name      = "myfile.lock"
  content        = "This blob is leased by Terraform"
  lease_duration = -1  # Infinite lease (default)
}
```

## Argument Reference

- `storage_account` (Required) - The name of the Azure Storage Account where the blob will be created.
- `container_name` (Required) - The name of the container where the blob will be created. The container will be created if it doesn't exist.
- `blob_name` (Required) - The name of the blob to create and lease.
- `content` (Optional) - The content to write to the blob. Defaults to "managed by terraform-provider-blobleas".
- `lease_duration` (Optional) - The lease duration in seconds. Use -1 for infinite lease (default), or 15-60 for time-limited lease.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

- `id` - The resource identifier in the format `storage_account/container_name/blob_name`.
- `lease_id` - The unique lease ID assigned to the blob.
- `blob_url` - The full URL of the blob.
- `etag` - The ETag of the blob.
- `lease_state` - The current lease state of the blob (e.g., "leased", "available").

## Import

Blob leases can be imported using the storage account, container name, and blob name:

```
terraform import blobleas_blob_lease.example mystorageaccount/mycontainer/myfile.lock
```

Note: When importing, the lease_id will be unknown and lease management may not work properly until the next apply.