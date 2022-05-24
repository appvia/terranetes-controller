variable "bucket" {
  description = "Name of the bucket to the create"
  type        = string
}

variable "location" {
  description = "Location of the bucket"
  type        = string
  default     = "europe-west2"
}
