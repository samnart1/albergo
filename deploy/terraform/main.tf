terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_sql_database_instance" "main" {
  name             = "${var.name}-db"
  database_version = "POSTGRES_18"
  region           = var.region

  # prod keeps this on
  deletion_protection = false

  settings {
    tier    = var.db_tier
    edition = "ENTERPRISE"

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
    }

    ip_configuration {
      ipv4_enabled = false
      # instance needs no pub addr?
      private_network = var.network_id
    }
  }
}

resource "google_sql_database" "albergo" {
  name     = var.name
  instance = google_sql_database_instance.main.name
}

resource "google_sql_user" "app" {
  name     = var.name
  instance = google_sql_database_instance.main.name
  password = var.db_password
}

resource "google_secret_manager_secret" "database_url" {
  secret_id = "${var.name}-database-url"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "database_url" {
  secret = google_secret_manager_secret.database_url.id

  secret_data = "postgres://${google_sql_user.app.name}:${var.db_password}@/${google_sql_database.albergo.name}?host=/cloudsql/${google_sql_database_instance.main.connection_name}"
}

resource "google_secret_manager_secret" "staff_api_key" {
  secret_id = "${var.name}-staff-api-key"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "staff_api_key" {
  secret      = google_secret_manager_secret.staff_api_key.id
  secret_data = var.staff_api_key
}

resource "google_service_account" "app" {
  account_id   = "${var.name}-app"
  display_name = "albergo runtime"
}

resource "google_project_iam_member" "cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.app.email}"
}

resource "google_secret_manager_secret_iam_member" "database_url" {
  secret_id = google_secret_manager_secret.database_url.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app.email}"
}

resource "google_secret_manager_secret_iam_member" "staff_api_key" {
  secret_id = google_secret_manager_secret.staff_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app.email}"
}

resource "google_cloud_run_v2_service" "api" {
  name                = "${var.name}-api"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account = google_service_account.app.email

    scaling {
      min_instance_count = 0
      # instance holds its pool of 10
      # cap * 10 < cloud sql connection limit
      max_instance_count = 4
    }

    containers {
      image = var.image

      ports {
        container_port = 8080
      }

      env {
        name  = "APP_ENV"
        value = "production"
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "STAFF_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.staff_api_key.secret_id
            version = "latest"
          }
        }
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
      }

      startup_probe {
        http_get {
          path = "/readyz"
        }
        initial_delay_seconds = 5
        failure_threshold     = 10
        period_seconds        = 3
      }

      liveness_probe {
        http_get {
          path = "/healthz"
        }
        period_seconds = 30
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.main.connection_name]
      }
    }
  }
}

resource "google_cloud_run_v2_service" "worker" {
  name                = "${var.name}-worker"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_INTERNAL_ONLY"
  deletion_protection = false

  template {
    service_account = google_service_account.app.email

    scaling {
      # one instance at a time
      min_instance_count = 1
      max_instance_count = 1
    }

    containers {
      image   = var.image
      command = ["/worker"]

      env {
        name  = "APP_ENV"
        value = "production"
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "STAFF_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.staff_api_key.secret_id
            version = "latest"
          }
        }
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
        cpu_idle = false
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.main.connection_name]
      }
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "public_api" {
  name     = google_cloud_run_v2_service.api.name
  location = google_cloud_run_v2_service.api.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
