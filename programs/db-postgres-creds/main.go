package main

import (
	"fmt"

	"github.com/pulumi/pulumi-postgresql/sdk/v3/go/postgresql"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/shivanshs9/iac-pulumi/components/aws/secret"
	"github.com/shivanshs9/iac-pulumi/components/postgres"
	"github.com/shivanshs9/iac-pulumi/components/utils"
)

type pgProviderArg struct {
	Host              pulumi.StringInput `required:"" json:"host"`
	SuperuserName     pulumi.StringInput `required:"" json:"superuserName"`
	SuperuserPassword pulumi.StringInput `required:"" secret:"superuserPassword"`
	Port              int                `required:"" json:"port"`
	DisableSSL        bool               `json:"disableSSL"`
}

type pgUserArg struct {
	Username string `json:"username"`
	Login    bool   `json:"login"`
}

type pgConfig struct {
	Database       string      `json:"database" required:""`
	Users          []pgUserArg `json:"users"`
	ExportAsSecret bool        `json:"exportAsSecret"`

	provider pgProviderArg
}

func (cfg *pgConfig) provisionDatabase(ctx *pulumi.Context, provider *postgresql.Provider) (*postgres.PostgresDBResource, error) {
	dbProps := postgres.PostgresDbProps{
		Database: cfg.Database,
	}
	res, err := postgres.NewPostgresDatabase(ctx, cfg.Database, dbProps, pulumi.Provider(provider))
	if err != nil {
		return res, err
	}

	return res, nil
}

func (cfg *pgConfig) provisionLoginUsers(ctx *pulumi.Context, provider *postgresql.Provider) (*postgres.PostgresUsersResource, error) {
	userProps := make([]postgres.PostgresUserProps, len(cfg.Users))
	for i, user := range cfg.Users {
		userProps[i] = postgres.PostgresUserProps{
			Username:   user.Username,
			Login:      user.Login,
			AssumeRole: pulumi.Sprintf("%s-rw", cfg.Database),
		}
	}
	res, err := postgres.NewPostgresUsers(ctx, cfg.Database, userProps, pulumi.Provider(provider))
	if err != nil {
		return res, err
	}
	return res, nil
}

func (cfg *pgConfig) genCredsMap(usersRes *postgres.PostgresUsersResource, i int) pulumi.StringMap {
	return pulumi.StringMap{
		"username": usersRes.Users[i].Name,
		"password": usersRes.Users[i].Password.Elem().ToStringOutput(),
		"database": pulumi.String(cfg.Database),
		"host":     cfg.provider.Host,
		"port":     pulumi.Sprintf("%d", cfg.provider.Port),
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := &pgConfig{}
		if err := utils.ExtractConfig(ctx, "pg", cfg); err != nil {
			return err
		}
		cfg.provider = pgProviderArg{}
		if err := utils.ExtractConfig(ctx, "provider", &cfg.provider); err != nil {
			return err
		}
		providerArgs := &postgresql.ProviderArgs{
			Host:     cfg.provider.Host,
			Username: cfg.provider.SuperuserName,
			Password: cfg.provider.SuperuserPassword,
			Port:     pulumi.IntPtr(cfg.provider.Port),
		}
		if cfg.provider.DisableSSL {
			providerArgs.Sslmode = pulumi.String("disable")
		}
		provider, err := postgresql.NewProvider(ctx, "postgresql", providerArgs)
		if err != nil {
			return err
		}
		// Provision database
		dbRes, err := cfg.provisionDatabase(ctx, provider)
		if err != nil {
			args := &pulumi.LogArgs{
				Resource: dbRes,
			}
			ctx.Log.Error(err.Error(), args)
			return err
		}

		if len(cfg.Users) > 0 {
			usersRes, err := cfg.provisionLoginUsers(ctx, provider)
			if err != nil {
				args := &pulumi.LogArgs{
					Resource: usersRes,
				}
				wrappedErr := fmt.Errorf("failed to create user '%s': %w", usersRes.FailedUser, err)
				ctx.Log.Error(wrappedErr.Error(), args)
			}
			// expose each user creds in independent secret
			if cfg.ExportAsSecret {
				for i, user := range cfg.Users {
					secret, err := secret.NewAWSSecret(ctx, secret.AWSSecretProps{
						Name:         fmt.Sprintf("pg-%s-user-%s", cfg.Database, user.Username),
						Type:         secret.DBCreds,
						InitialValue: cfg.genCredsMap(usersRes, i),
					})
					if err != nil {
						return fmt.Errorf("failed to create secret for user %s: %w", user.Username, err)
					}
					ctx.Export(fmt.Sprintf("secret-%s", user.Username), pulumi.StringMap{
						"secretId": secret.Secret.ID(),
					})
				}
			} else {
				for i, user := range cfg.Users {
					ctx.Export(user.Username, cfg.genCredsMap(usersRes, i))
				}
			}
		}
		ctx.Export("database", pulumi.String(cfg.Database))

		return nil
	})
}
