package blobclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"
)

// AzureBlobLeaseClient is the main client for Azure Blob Storage lease operations
type AzureBlobLeaseClient struct {
	credential azcore.TokenCredential
}

// NewAzureBlobLeaseClient creates a new Azure Blob Storage lease client with Azure authentication
func NewAzureBlobLeaseClient() (*AzureBlobLeaseClient, error) {
    clientID := os.Getenv("ARM_CLIENT_ID")
    clientSecret := os.Getenv("ARM_CLIENT_SECRET")
    tenantID := os.Getenv("ARM_TENANT_ID")

    var cred azcore.TokenCredential
    var err error

    if clientID != "" && clientSecret != "" && tenantID != "" {
        // Build credential from ARM_* variables (Terraform style)
        cred, err = azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
        if err != nil {
            return nil, fmt.Errorf("failed to create ClientSecretCredential: %w", err)
        }
    } else {
        // Fallback: standard Azure SDK auth chain
        cred, err = azidentity.NewDefaultAzureCredential(nil)
        if err != nil {
            return nil, fmt.Errorf("failed to create DefaultAzureCredential: %w", err)
        }
    }

    return &AzureBlobLeaseClient{
        credential: cred,
    }, nil
}

// CreateBlobClient creates a blob client for the specified storage account
func (c *AzureBlobLeaseClient) CreateBlobClient(storageAccount string) (*azblob.Client, error) {
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", storageAccount)
	client, err := azblob.NewClient(serviceURL, c.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob client for %s: %w", storageAccount, err)
	}
	return client, nil
}

// BlobLeaseConfig holds configuration for blob lease operations
type BlobLeaseConfig struct {
	StorageAccount string
	ContainerName  string
	BlobName       string
	Content        []byte
	LeaseID        string
	LeaseDuration  int32 // -1 for infinite, 15-60 for seconds (default: -1)
}

// BlobLeaseResult represents the result of blob lease operations
type BlobLeaseResult struct {
	LeaseID    string
	BlobURL    string
	ETag       string
	LeaseState string
}

// CreateBlobWithLease creates a blob and immediately leases it
func (c *AzureBlobLeaseClient) CreateBlobWithLease(ctx context.Context, config BlobLeaseConfig) (*BlobLeaseResult, error) {
	// Create blob client
	blobClient, err := c.CreateBlobClient(config.StorageAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob client: %w", err)
	}

	// Create container if it doesn't exist
	containerClient := blobClient.ServiceClient().NewContainerClient(config.ContainerName)
	_, err = containerClient.Create(ctx, nil)
	if err != nil {
		// Container might already exist, continue
		if !strings.Contains(err.Error(), "ContainerAlreadyExists") {
			return nil, fmt.Errorf("failed to create container %s: %w", config.ContainerName, err)
		}
	}

	// Upload blob
	blobClientRef := containerClient.NewBlockBlobClient(config.BlobName)
	uploadResp, err := blobClientRef.UploadBuffer(ctx, config.Content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload blob %s: %w", config.BlobName, err)
	}

	// Acquire lease
	leaseClient, err := lease.NewBlobClient(blobClientRef, &lease.BlobClientOptions{
		LeaseID: &config.LeaseID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create lease client: %w", err)
	}

	leaseDuration := config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = -1 // Default to infinite
	}
	acquireResp, err := leaseClient.AcquireLease(ctx, leaseDuration, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lease on blob %s: %w", config.BlobName, err)
	}

	return &BlobLeaseResult{
		LeaseID:    *acquireResp.LeaseID,
		BlobURL:    blobClientRef.URL(),
		ETag:       string(*uploadResp.ETag),
		LeaseState: "leased",
	}, nil
}

// RenewBlobLease renews an existing blob lease
func (c *AzureBlobLeaseClient) RenewBlobLease(ctx context.Context, config BlobLeaseConfig) (*BlobLeaseResult, error) {
	// Create blob client
	blobClient, err := c.CreateBlobClient(config.StorageAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob client: %w", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(config.ContainerName)
	blobClientRef := containerClient.NewBlockBlobClient(config.BlobName)

	// Check if lease exists and is active
	props, err := blobClientRef.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	if props.LeaseState == nil || *props.LeaseState != "leased" {
		// Lease is broken/expired, try to acquire a new one
		leaseClient, err := lease.NewBlobClient(blobClientRef, &lease.BlobClientOptions{
			LeaseID: &config.LeaseID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create lease client: %w", err)
		}

		leaseDuration := config.LeaseDuration
	if leaseDuration == 0 {
		leaseDuration = -1 // Default to infinite
	}
	acquireResp, err := leaseClient.AcquireLease(ctx, leaseDuration, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to re-acquire lease on blob %s: %w", config.BlobName, err)
		}

		return &BlobLeaseResult{
			LeaseID:    *acquireResp.LeaseID,
			BlobURL:    blobClientRef.URL(),
			ETag:       string(*props.ETag),
			LeaseState: "leased",
		}, nil
	}

	// Renew existing lease
	leaseClient, err := lease.NewBlobClient(blobClientRef, &lease.BlobClientOptions{
		LeaseID: &config.LeaseID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create lease client: %w", err)
	}

	renewResp, err := leaseClient.RenewLease(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to renew lease on blob %s: %w", config.BlobName, err)
	}

	return &BlobLeaseResult{
		LeaseID:    *renewResp.LeaseID,
		BlobURL:    blobClientRef.URL(),
		ETag:       string(*props.ETag),
		LeaseState: "leased",
	}, nil
}

// ReleaseBlobLease releases a blob lease and optionally deletes the blob
func (c *AzureBlobLeaseClient) ReleaseBlobLease(ctx context.Context, config BlobLeaseConfig, deleteBlob bool) error {
	// Create blob client
	blobClient, err := c.CreateBlobClient(config.StorageAccount)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %w", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(config.ContainerName)
	blobClientRef := containerClient.NewBlockBlobClient(config.BlobName)

	// Release lease
	leaseClient, err := lease.NewBlobClient(blobClientRef, &lease.BlobClientOptions{
		LeaseID: &config.LeaseID,
	})
	if err != nil {
		return fmt.Errorf("failed to create lease client: %w", err)
	}

	_, err = leaseClient.ReleaseLease(ctx, nil)
	if err != nil {
		// If lease doesn't exist or is already broken, continue to deletion
		if !strings.Contains(err.Error(), "LeaseNotPresentWithBlobOperation") &&
			!strings.Contains(err.Error(), "LeaseIdMismatchWithBlobOperation") {
			return fmt.Errorf("failed to release lease on blob %s: %w", config.BlobName, err)
		}
	}

	// Delete blob if requested
	if deleteBlob {
		_, err = blobClientRef.Delete(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to delete blob %s: %w", config.BlobName, err)
		}
	}

	return nil
}

// BlobExists checks if a blob exists
func (c *AzureBlobLeaseClient) BlobExists(ctx context.Context, storageAccount, containerName, blobName string) (bool, error) {
	// Create blob client
	blobClient, err := c.CreateBlobClient(storageAccount)
	if err != nil {
		return false, fmt.Errorf("failed to create blob client: %w", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(containerName)
	blobClientRef := containerClient.NewBlockBlobClient(blobName)

	_, err = blobClientRef.GetProperties(ctx, nil)
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "BlobNotFound") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check blob existence: %w", err)
	}

	return true, nil
}

// GetBlobLeaseState gets the current lease state of a blob
func (c *AzureBlobLeaseClient) GetBlobLeaseState(ctx context.Context, storageAccount, containerName, blobName string) (*BlobLeaseResult, error) {
	// Create blob client
	blobClient, err := c.CreateBlobClient(storageAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob client: %w", err)
	}

	containerClient := blobClient.ServiceClient().NewContainerClient(containerName)
	blobClientRef := containerClient.NewBlockBlobClient(blobName)

	props, err := blobClientRef.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	leaseState := "available"
	if props.LeaseState != nil {
		leaseState = string(*props.LeaseState)
	}

	return &BlobLeaseResult{
		BlobURL:    blobClientRef.URL(),
		ETag:       string(*props.ETag),
		LeaseState: leaseState,
	}, nil
}

// StartLeaseRenewal starts a background process to automatically renew the lease
func (c *AzureBlobLeaseClient) StartLeaseRenewal(ctx context.Context, config BlobLeaseConfig, renewInterval time.Duration) error {
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, err := c.RenewBlobLease(ctx, config)
			if err != nil {
				return fmt.Errorf("failed to renew lease during background renewal: %w", err)
			}
		}
	}
}