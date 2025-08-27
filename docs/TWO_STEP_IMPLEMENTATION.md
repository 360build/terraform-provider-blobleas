# Azure Key Vault CA Terraform Provider - Two-Step Implementation

This implementation provides a two-step approach to certificate management using Azure Key Vault Certificate Authority operations.

## Resources

### `azurevaultca_certificate_request`

Creates a Certificate Signing Request (CSR) in Azure Key Vault. This resource generates a private key and CSR but does not sign the certificate.

**Use cases:**
- When you want to control the signing process separately
- For certificate renewal scenarios where you want to reuse the same private key
- When you need to sign the same CSR with different CAs or validity periods

**Example:**
```hcl
resource "azurevaultca_certificate_request" "web_csr" {
  cert_vault       = "https://myvault.vault.azure.net"
  certificate_name = "web-server-cert"
  
  subject = {
    common_name = "www.example.com"
    organization = "My Company"
    country = "US"
  }
  
  subject_alternative_names = {
    dns_names = ["www.example.com", "api.example.com"]
  }
  
  key_size   = 2048
  exportable = false
}
```

### `azurevaultca_signed_certificate`

Signs an existing Certificate Signing Request using a root CA from Azure Key Vault. This resource takes a CSR and produces a fully signed certificate.

**Use cases:**
- Signing CSRs created by `azurevaultca_certificate_request`
- Implementing different signing policies for different environments
- Certificate renewal without regenerating private keys

**Example:**
```hcl
resource "azurevaultca_signed_certificate" "web_cert" {
  certificate_request_id = azurevaultca_certificate_request.web_csr.id
  
  root_vault = "https://ca-vault.vault.azure.net"
  root_cert  = "internal-ca-cert"
  
  validity_months = 12
}
```

### `azurevaultca_server_certificate` (Legacy)

The original single-step resource that creates and signs a certificate in one operation. Still available for backward compatibility and simple use cases.

## Benefits of Two-Step Approach

1. **Crash Recovery**: If Terraform crashes during the signing process, the CSR is preserved and can be signed again.

2. **Flexibility**: The same CSR can be signed multiple times with different parameters:
   - Different validity periods
   - Different root CAs
   - Different environments

3. **Separation of Concerns**: CSR creation and signing are separate operations, making troubleshooting easier.

4. **Reusability**: CSRs can be reused for certificate renewal scenarios.

5. **Testing**: You can create test certificates with short validity periods and production certificates with longer validity periods from the same CSR.

## Migration from Single-Step

If you're currently using `azurevaultca_server_certificate`, you can migrate to the two-step approach:

**Before:**
```hcl
resource "azurevaultca_server_certificate" "web_server" {
  root_vault = "https://ca-vault.vault.azure.net"
  root_cert  = "internal-ca-cert"
  
  cert_vault       = "https://app-vault.vault.azure.net"
  certificate_name = "web-server-cert"
  
  subject = {
    common_name = "www.example.com"
  }
  
  validity_months = 12
}
```

**After:**
```hcl
resource "azurevaultca_certificate_request" "web_server_csr" {
  cert_vault       = "https://app-vault.vault.azure.net"
  certificate_name = "web-server-cert"
  
  subject = {
    common_name = "www.example.com"
  }
}

resource "azurevaultca_signed_certificate" "web_server" {
  certificate_request_id = azurevaultca_certificate_request.web_server_csr.id
  
  root_vault = "https://ca-vault.vault.azure.net"
  root_cert  = "internal-ca-cert"
  
  validity_months = 12
}
```

## Error Handling

### CSR Resource Deletion
When a `azurevaultca_certificate_request` resource is deleted:
1. The certificate operation is cancelled (if still pending)
2. The certificate object is deleted from Azure Key Vault
3. Any dependent `azurevaultca_signed_certificate` resources will need to be recreated

### Signed Certificate Deletion
When a `azurevaultca_signed_certificate` resource is deleted:
1. The resource is removed from Terraform state
2. The underlying certificate remains in Azure Key Vault
3. The original CSR remains intact and can be signed again

This behavior allows for certificate renewal scenarios where you want to replace a signed certificate but keep the original CSR.

## Dependencies

The two-step approach uses Terraform's dependency system:
```hcl
resource "azurevaultca_signed_certificate" "example" {
  certificate_request_id = azurevaultca_certificate_request.example_csr.id
  # This creates an implicit dependency - the CSR must exist before signing
}
```

If you delete the CSR resource, Terraform will automatically plan to recreate the signed certificate as well.

## Troubleshooting

### "Certificate operation not found"
This usually means the CSR has been cancelled or completed outside of Terraform. Check the Azure Key Vault portal to see the certificate status.

### "Certificate request ID not found"
The referenced `azurevaultca_certificate_request` resource doesn't exist or has been deleted. Ensure the CSR resource is created first.

### Soft Delete Issues
Azure Key Vault has soft delete enabled by default. If you delete and recreate certificates with the same name quickly, you might encounter conflicts. The provider attempts to handle this automatically, but you may need to wait or manually purge deleted certificates in some cases.
