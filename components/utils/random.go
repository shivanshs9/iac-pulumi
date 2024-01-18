package utils

import (
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewRandomPassword(ctx *pulumi.Context, name string, len int, opts ...pulumi.ResourceOption) (pulumi.StringOutput, error) {
	passwd, err := random.NewRandomPassword(ctx, name, &random.RandomPasswordArgs{
		Length:          pulumi.Int(len),
		OverrideSpecial: pulumi.String("!#$%&*()-_=+[]{}<>:?"),
	}, opts...)
	if err != nil {
		return pulumi.StringOutput{}, err
	}
	return passwd.Result, nil
}
