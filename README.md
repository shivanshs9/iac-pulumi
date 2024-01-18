## IAC Pulumi modules

This repo contains some of my reusable IaC modules for Public use.

### AWS Components

- [AWS Secret Manager](./components/aws/secret/)

### Postgres Components

- [PG Database & Users](./components/postgres/)

### Programs

1. [Postgres Creds](./programs/db-postgres-creds/): Managed Postgres DB and login users, optionally exposing them in AWS Secret.

### Prerequisites

1. Pulumi - [Installation Guide](https://www.pulumi.com/docs/install/)
2. Login Pulumi to backend.
   > For testing, it's fine to use local statefile - `pulumi login --local`
