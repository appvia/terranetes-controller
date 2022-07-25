resource "azurerm_resource_group" "this" {
  name = "terranetes-controller-e2e"

  location = "West Europe"
}

resource "azurerm_storage_account" "this" {
  name = var.bucket

  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_storage_container" "this" {
  name = "terranetes-controller-e2e"

  storage_account_name  = azurerm_storage_account.this.name
  container_access_type = "private"
}

