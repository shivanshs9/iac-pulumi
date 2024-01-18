package postgres

import (
	"fmt"

	postgresql "github.com/pulumi/pulumi-postgresql/sdk/v3/go/postgresql"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/shivanshs9/iac-pulumi/components/utils"
)

type PostgresUsersResource struct {
	pulumi.ResourceState

	Users      []*postgresql.Role
	FailedUser string
}

type PostgresUserProps struct {
	Username   string             `json:"username"`
	Password   pulumi.StringInput `json:"password"`
	AssumeRole pulumi.StringInput `json:"assumeRole"`
	Login      bool               `json:"login"`
}

func (props *PostgresUserProps) fillRuntimeInputs(ctx *pulumi.Context, res *PostgresUsersResource) (err error) {
	if props.Password == nil {
		props.Password, err = utils.NewRandomPassword(
			ctx, fmt.Sprintf("%s-%s", props.Username, "password"), 16, pulumi.Parent(res))
	}
	return
}

func (r *PostgresUsersResource) provision(ctx *pulumi.Context, name string, props *PostgresUserProps) error {
	if err := props.fillRuntimeInputs(ctx, r); err != nil {
		return err
	}

	role, err := postgresql.NewRole(ctx, fmt.Sprintf("%s-%s", name, props.Username), &postgresql.RoleArgs{
		Name:       pulumi.String(props.Username),
		Password:   props.Password,
		Login:      pulumi.BoolPtr(props.Login),
		AssumeRole: props.AssumeRole,
		Roles:      pulumi.StringArray{props.AssumeRole},
	}, pulumi.Parent(r))
	if err != nil {
		return err
	}
	r.Users = append(r.Users, role)
	return nil
}

func NewPostgresUsers(ctx *pulumi.Context, name string, props []PostgresUserProps, opts ...pulumi.ResourceOption) (*PostgresUsersResource, error) {
	resource := &PostgresUsersResource{}
	if err := ctx.RegisterComponentResource("ss9:postgres:users", name, resource, opts...); err != nil {
		return nil, err
	}
	for _, prop := range props {
		err := resource.provision(ctx, name, &prop)
		if err != nil {
			resource.FailedUser = prop.Username
			return resource, err
		}
	}

	outputRoles := make([]pulumi.MapInput, len(resource.Users))
	for _, role := range resource.Users {
		outputRoles = append(outputRoles, pulumi.Map{
			"username": role.Name,
			"password": role.Password,
		})
	}
	ctx.RegisterResourceOutputs(resource, pulumi.Map{
		"users": pulumi.MapArray(outputRoles),
	})
	return resource, nil
}
