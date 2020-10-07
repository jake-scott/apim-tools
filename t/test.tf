variable "rg_name" {
    type = string
}

variable "region" {
    type = string
}

variable "rand" {
    type = string
}

output "resourcegroup" {
    value = azurerm_resource_group.test.name
}

output "apimname" {
    value = azurerm_api_management.test.name
}

provider "azurerm" {
    version = "~>2.29.0"
    features {}
}

resource "azurerm_resource_group" "test" {
    name = var.rg_name
    location = var.region
}

resource "azurerm_api_management" "test" {
  name                = "test-${var.rand}"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
  publisher_name      = "CI Testing"
  publisher_email     = "ci@jakethesnake.dev"

  sku_name = "Developer_1"
}
