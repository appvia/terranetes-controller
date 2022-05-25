output "bucket_name" {
  description = "The name of the bucket"
  value       = try(module.bucket.bucket, "")
}
