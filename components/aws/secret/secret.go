package secret

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/kms"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type SecretType string

const (
	DBCreds    SecretType = "db"
	MongoCreds SecretType = "mongo"
)

type AWSSecretProps struct {
	Name         string
	Type         SecretType
	InitialValue pulumi.StringMapInput
}

func (props AWSSecretProps) String() string {
	descriptionType := map[SecretType]string{
		DBCreds:    "database credentials",
		MongoCreds: "mongo connection details",
	}
	return fmt.Sprintf("Secret %s to store %s", props.Name, descriptionType[props.Type])
}

type AWSSecret struct {
	pulumi.ResourceState

	Secret *secretsmanager.Secret
}

func (s *AWSSecret) newSecret(ctx *pulumi.Context, props *AWSSecretProps) (*secretsmanager.Secret, error) {
	tags := pulumi.StringMap{
		"Pulumi": pulumi.String("true"),
	}
	var kmsKeyId string
	kmsKeyAlias, ok := ctx.GetConfig("secret:kms_alias")
	if ok {
		kmsKey, err := kms.LookupAlias(ctx, &kms.LookupAliasArgs{
			Name: kmsKeyAlias,
		})
		if err != nil {
			return nil, err
		}
		kmsKeyId = kmsKey.TargetKeyId
	}
	tags["secret:type"] = pulumi.String(props.Type)

	args := &secretsmanager.SecretArgs{
		Name:        pulumi.Sprintf("%s-%s", props.Type, props.Name),
		Description: pulumi.String(props.String()),
		Tags:        tags,
	}
	if kmsKeyId != "" {
		args.KmsKeyId = pulumi.String(kmsKeyId)
	}
	secret, err := secretsmanager.NewSecret(ctx, fmt.Sprintf("secret-%s", props.Name), args, pulumi.Parent(s))
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (s *AWSSecret) provision(ctx *pulumi.Context, props *AWSSecretProps) error {
	secret, err := s.newSecret(ctx, props)
	if err != nil {
		return err
	}

	s.Secret = secret
	outputs := pulumi.Map{
		"secretArn": secret.Arn,
	}
	if props.InitialValue != nil {
		secVersion := props.InitialValue.ToStringMapOutput().ApplyT(func(val map[string]string) (pulumi.StringOutput, error) {
			secretDict, err := json.Marshal(val)
			if err != nil {
				return pulumi.StringOutput{}, fmt.Errorf("failed to marshal secret data into json: %w", err)
			}
			secVersion, err := secretsmanager.NewSecretVersion(ctx, fmt.Sprintf("secretversion-initial-%s", props.Name), &secretsmanager.SecretVersionArgs{
				SecretId:     secret.Arn,
				SecretString: pulumi.String(string(secretDict)),
			}, pulumi.Parent(s))
			if err != nil {
				return pulumi.StringOutput{}, err
			}
			return secVersion.VersionId, nil
		}).(pulumi.StringOutput)
		outputs["secretVersion"] = secVersion
	}
	ctx.RegisterResourceOutputs(s, outputs)
	return nil
}

func NewAWSSecret(ctx *pulumi.Context, props AWSSecretProps, opts ...pulumi.ResourceOption) (*AWSSecret, error) {
	secret := &AWSSecret{}
	err := ctx.RegisterComponentResource("ss9:aws:secretmanager:secret", props.Name, secret, opts...)
	if err != nil {
		return nil, err
	}
	err = secret.provision(ctx, &props)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
