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
	Host              pulumi.StringInput `json:"host"`
	SuperuserName     pulumi.StringInput `json:"superuserName"`
	SuperuserPassword pulumi.StringInput `json:"superuserPassword"`
	Port              pulumi.IntInput    `json:"port"`
}

type pgConfig struct {
	Database       postgres.PostgresDbProps `json:"database"`
	Provider       pgProviderArg            `json:"provider"`
	Users          []string                 `json:"users"`
	ExportAsSecret bool                     `json:"exportAsSecret"`
}

func (cfg *pgConfig) provisionDatabase(ctx *pulumi.Context, provider *postgresql.Provider) (*postgres.PostgresDBResource, error) {
	res, err := postgres.NewPostgresDatabase(ctx, cfg.Database.Database, cfg.Database, pulumi.Provider(provider))
	if err != nil {
		return res, err
	}

	return res, nil
}

func (cfg *pgConfig) provisionLoginUsers(ctx *pulumi.Context, provider *postgresql.Provider) (*postgres.PostgresUsersResource, error) {
	userProps := make([]postgres.PostgresUserProps, len(cfg.Users))
	for i, username := range cfg.Users {
		userProps[i] = postgres.PostgresUserProps{
			Username:   username,
			Login:      true,
			AssumeRole: pulumi.Sprintf("%s-rw", cfg.Database.Database),
		}
	}
	res, err := postgres.NewPostgresUsers(ctx, cfg.Database.Database, userProps, pulumi.Provider(provider))
	if err != nil {
		return res, err
	}
	return res, nil
}

func (cfg *pgConfig) genCredsMap(usersRes *postgres.PostgresUsersResource, i int) pulumi.StringMap {
	return pulumi.StringMap{
		"username": usersRes.Users[i].Name,
		"password": usersRes.Users[i].Password.Elem().ToStringOutput(),
		"database": pulumi.String(cfg.Database.Database),
		"host":     cfg.Provider.Host,
		"port":     pulumi.Sprintf("%d", cfg.Provider.Port),
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := &pgConfig{}
		if err := utils.ExtractConfig(ctx, "pg", cfg); err != nil {
			return err
		}
		provider, err := postgresql.NewProvider(ctx, "postgresql", &postgresql.ProviderArgs{
			Host:     cfg.Provider.Host,
			Username: cfg.Provider.SuperuserName,
			Password: cfg.Provider.SuperuserPassword,
			Port:     cfg.Provider.Port,
		})
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
					_, err := secret.NewAWSSecret(ctx, secret.AWSSecretProps{
						Name:         fmt.Sprintf("pg-%s-user-%s", cfg.Database.Database, user),
						Type:         secret.DBCreds,
						InitialValue: cfg.genCredsMap(usersRes, i),
					})
					if err != nil {
						return fmt.Errorf("failed to create secret for user %s: %w", user, err)
					}
				}
			} else {
				for i, user := range cfg.Users {
					ctx.Export(user, cfg.genCredsMap(usersRes, i))
				}
			}
		}
		ctx.Export("database", pulumi.String(cfg.Database.Database))

		return nil
	})
}