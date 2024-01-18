package postgres

import (
	"encoding/json"
	"fmt"

	postgresql "github.com/pulumi/pulumi-postgresql/sdk/v3/go/postgresql"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type PostgresUserPermission string

const (
	ReadWrite PostgresUserPermission = "rw"
	ReadOnly  PostgresUserPermission = "ro"
)

type PostgresDbRoleProps struct {
	Permission PostgresUserPermission `json:"permission"`
}

type PostgresDbProps struct {
	Database string                `json:"database"`
	DbRoles  []PostgresDbRoleProps `json:"dbRoles"`
}

func (i PostgresDbProps) String() string {
	jsonBytes, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return string(jsonBytes)
}

func (props *PostgresDbRoleProps) fillRuntimeInputs(ctx *pulumi.Context, res *PostgresDBResource) (err error) {
	if props.Permission != ReadOnly && props.Permission != ReadWrite {
		return fmt.Errorf("invalid permission %s", props.Permission)
	}
	return
}

func (props *PostgresDbProps) fillRuntimeInputs(ctx *pulumi.Context, res *PostgresDBResource) error {
	if len(props.DbRoles) == 0 {
		props.DbRoles = []PostgresDbRoleProps{{Permission: ReadWrite}}
	}
	if len(props.DbRoles) > 2 {
		return fmt.Errorf("only 2 roles are supported, one read-write and other read-only")
	}
	for i := range props.DbRoles {
		if err := props.DbRoles[i].fillRuntimeInputs(ctx, res); err != nil {
			return err
		}
	}
	return nil
}

type PostgresDBResource struct {
	pulumi.ResourceState

	Roles []*postgresql.Role
	DB    *postgresql.Database
}

func (r *PostgresDBResource) provisionDB(ctx *pulumi.Context, namePrefix string, roleName pulumi.StringInput, props *PostgresDbProps) (db *postgresql.Database, err error) {
	// CREATE DATABASE $DB;
	db, err = postgresql.NewDatabase(ctx, fmt.Sprintf("%s-db", namePrefix), &postgresql.DatabaseArgs{
		Name:  pulumi.String(props.Database),
		Owner: roleName,
	}, pulumi.Parent(r))
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (r *PostgresDBResource) provision(ctx *pulumi.Context, namePrefix string, props *PostgresDbProps) error {
	if err := props.fillRuntimeInputs(ctx, r); err != nil {
		return err
	}
	var owner pulumi.StringInput = pulumi.String("postgres")
	r.Roles = make([]*postgresql.Role, len(props.DbRoles))
	for i, user := range props.DbRoles {
		role, err := r.provisionUser(ctx, namePrefix, user)
		if err != nil {
			return err
		}
		r.Roles[i] = role
		if user.Permission == ReadWrite {
			owner = role.Name
		}
	}
	db, err := r.provisionDB(ctx, namePrefix, owner, props)
	if err != nil {
		return err
	}
	r.DB = db
	for i, role := range r.Roles {
		if err := r.grantDBAccess(ctx, namePrefix, role.Name, props.DbRoles[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresDBResource) grantDBAccess(ctx *pulumi.Context, namePrefix string, roleName pulumi.StringOutput, userProps PostgresDbRoleProps) error {
	database := r.DB.Name
	if userProps.Permission == ReadOnly {
		// GRANT SELECT ON ALL TABLES IN SCHEMA public TO rouser
		if _, err := postgresql.NewGrant(ctx, fmt.Sprintf("%s-readOnlyTables", namePrefix), &postgresql.GrantArgs{
			Database:   database,
			ObjectType: pulumi.String("table"),
			Objects:    pulumi.StringArray{},
			Privileges: pulumi.StringArray{pulumi.String("SELECT")},
			Role:       roleName,
			Schema:     pulumi.String("public"),
		}, pulumi.Parent(r)); err != nil {
			return err
		}
		// GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO rouser;
		if _, err := postgresql.NewGrant(ctx, fmt.Sprintf("%s-readOnlySequences", namePrefix), &postgresql.GrantArgs{
			Database:   database,
			ObjectType: pulumi.String("sequence"),
			Objects:    pulumi.StringArray{},
			Privileges: pulumi.StringArray{pulumi.String("SELECT")},
			Role:       roleName,
			Schema:     pulumi.String("public"),
		}, pulumi.Parent(r)); err != nil {
			return err
		}
		// GRANT CONNECT ON DATABASE $DB TO rouser;
		if _, err := postgresql.NewGrant(ctx, fmt.Sprintf("%s-connectDatabase", namePrefix), &postgresql.GrantArgs{
			Database:   database,
			ObjectType: pulumi.String("database"),
			Privileges: pulumi.StringArray{pulumi.String("CONNECT")},
			Role:       roleName,
		}, pulumi.Parent(r)); err != nil {
			return err
		}
		// GRANT USAGE ON SCHEMA public TO rouser;
		if _, err := postgresql.NewGrant(ctx, fmt.Sprintf("%s-usageSchema", namePrefix), &postgresql.GrantArgs{
			Database:   database,
			ObjectType: pulumi.String("schema"),
			Privileges: pulumi.StringArray{pulumi.String("USAGE")},
			Role:       roleName,
			Schema:     pulumi.String("public"),
		}, pulumi.Parent(r)); err != nil {
			return err
		}
		// REVOKE CREATE ON SCHEMA public FROM PUBLIC;
		// _, err = postgresql.NewGrant(ctx, "revokePublic", &postgresql.GrantArgs{
		// 	Database:   pulumi.String(database),
		// 	ObjectType: pulumi.String("schema"),
		// 	Privileges: pulumi.StringArray{},
		// 	Role:       pulumi.String(roUsername),
		// 	Schema:     pulumi.String("public"),
		// })
		// if err != nil {
		// 	return nil, err
		// }
	}
	return nil
}

func (r *PostgresDBResource) provisionUser(ctx *pulumi.Context, name string, props PostgresDbRoleProps) (*postgresql.Role, error) {
	roleName := fmt.Sprintf("%s-%s", name, props.Permission)
	role, err := postgresql.NewRole(ctx, roleName, &postgresql.RoleArgs{
		Name:  pulumi.String(roleName),
		Login: pulumi.BoolPtr(false),
	}, pulumi.Parent(r))
	if err != nil {
		return nil, err
	}
	return role, nil
}

func NewPostgresDatabase(ctx *pulumi.Context, name string, props PostgresDbProps, opts ...pulumi.ResourceOption) (*PostgresDBResource, error) {
	/*
	* Following args need to be implictly set:
	* postgresql:host - (required) The address for the postgresql server connection. Can also be specified with the PGHOST environment variable.
	* postgresql:port - (optional) The port for the postgresql server connection. The default is 5432. Can also be specified with the PGPORT environment variable.
	* postgresql:username - (required) Username for the server connection. The default is postgres. Can also be specified with the PGUSER environment variable.
	* postgresql:password - (optional) Password for the server connection. Can also be specified with the PGPASSWORD environment variable.
	 */
	resource := &PostgresDBResource{}
	if err := ctx.RegisterComponentResource("ss9:postgres:database", name, resource, opts...); err != nil {
		return nil, err
	}
	err := resource.provision(ctx, name, &props)
	if err != nil {
		return resource, err
	}

	outputRoles := make([]pulumi.MapInput, len(resource.Roles))
	for _, role := range resource.Roles {
		outputRoles = append(outputRoles, pulumi.Map{
			"username": role.Name,
		})
	}
	ctx.RegisterResourceOutputs(resource, pulumi.Map{
		"database": resource.DB.Name,
		"users":    pulumi.MapArray(outputRoles),
	})
	return resource, nil
}
