# PgBeam Terraform Provider

Terraform provider for [PgBeam](https://pgbeam.com) — manage your globally
distributed PostgreSQL proxy infrastructure as code.

## Usage

```hcl
terraform {
  required_providers {
    pgbeam = {
      source = "sferarc/pgbeam"
    }
  }
}

provider "pgbeam" {
  api_token = var.pgbeam_api_token
}

resource "pgbeam_project" "main" {
  name   = "my-project"
  org_id = var.org_id
  region = "us-east-1"
}

resource "pgbeam_database" "primary" {
  project_id = pgbeam_project.main.id
  name       = "primary"
  host       = var.db_host
  port       = 5432
  database   = "mydb"
  username   = var.db_user
  password   = var.db_password
}
```

## Resources

| Resource               | Description                          |
| ---------------------- | ------------------------------------ |
| `pgbeam_project`       | PgBeam project                       |
| `pgbeam_database`      | PostgreSQL database connection       |
| `pgbeam_replica`       | Read replica configuration           |
| `pgbeam_custom_domain` | Custom domain for connection strings |
| `pgbeam_cache_rule`    | Query caching rule                   |
| `pgbeam_spend_limit`   | Budget controls                      |

## Authentication

Set the `PGBEAM_API_TOKEN` environment variable or configure the `api_token`
argument in the provider block.

## Documentation

Full usage guide at
[docs.pgbeam.com/terraform](https://docs.pgbeam.com/terraform).

## Development

```bash
go build -o terraform-provider-pgbeam
```

## License

Apache 2.0 — see [LICENSE](LICENSE).
