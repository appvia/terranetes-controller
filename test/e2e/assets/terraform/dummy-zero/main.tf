variable "sentence" {
  description = "The sentence to print"
  type        = string
  default     = "The sentence has not been set"
}

terraform {
  required_version = ">= 1.0"
}

output "sentence" {
  description = "A sentence used to test inputs to configurations"
  value       = var.sentence
}
