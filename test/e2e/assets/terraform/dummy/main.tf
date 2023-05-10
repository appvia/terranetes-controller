
resource "random_integer" "this" {
  min     = 1
  max     = 99999
}

terraform {
  required_version = ">= 1.0"

  required_providers {
    time = {
      source = "hashicorp/time"
      version = "0.9.1"
    }
  }
}

output "number" {
  description = "The random number generated"
  value = random_integer.this.result
}
