variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "europe-west3"
}

variable "name" {
  type    = string
  default = "albergo"
}

variable "image" {
  type        = string
  description = "Fully qualified image, for example europe-west3-docker.pkg.dev/PROJECT/albergo/api:SHA"
}

variable "network_id" {
  type        = string
  description = "VPC the Cloud SQL instance attaches to"
}

variable "db_tier" {
  type    = string
  default = "db-g1-small"
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "staff_api_key" {
  type      = string
  sensitive = true
}
