## Postgres DB and Users

This program provisions:

1. Postgres DB (`pg:database`)
2. Read-Write non-login role for the DB (default name: `${DBNAME}-rw`)
3. Login Users which can assume the above role (`pg:users`)
4. Random Login Password for each user
5. Expose credentials via Secret Manager (`pg:exportAsSecret` needs to be true)

## How to deploy?

1. Complete [pre-requisites](/README.md#prerequisites)
2. Sample stack config is provided in [Pulumi.dev.yaml](./Pulumi.dev.yaml), update the DB Host in it.
3. Superuser password needs to be set as secret:

```bash
pulumi config -s dev set --secret provider:superuserPassword <value>
```

4. To Deploy, run:

```bash
pulumi up -s dev
```

5. If `pg:exportAsSecret` is true, creds will be exposed as AWS Secret. Refer to IDs from the output of the program.
6. If above var is false, then creds are exposed as regular Pulumi output. To print them (along with secret password):

```bash
pulumi stack output -s dev -j --show-secrets
```

## Rotate Passwords without downtime

The idea is to not update existing user's password, since it'll cause a downtime. So first create a new login user and update the secrets in application, before deleting the current one.

Simple steps to achieve this:

1. Assume currently you've one user named `tom` in your stack config:

```yaml
pg:users:
  - username: tom
    login: true
```

> App is referencing secret ID for tom user in its env variables.

2. And you want to create a new user `jerry`, then append it in the list:

```yaml
pg:users:
  - username: tom
    login: true
  - username: jerry
    login: true
```

> Now there will be 2 login users, and consequently 2 AWS Secret IDs in the output.

3. Update the app env to point to the new secret.
4. Delete the `tom` user from the config - this will remove the user from postgres DB and delete the secret.
