package main

import (
	"os"
	"path"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/iamrole"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/lambdafunction"
	awsprovider "github.com/cdktf/cdktf-provider-aws-go/aws/v10/provider"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/s3bucket"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/s3bucketobject"
	"github.com/hashicorp/terraform-cdk-go/cdktf"
)

func AWSLambda(scope constructs.Construct, id string) cdktf.TerraformStack {
	stack := cdktf.NewTerraformStack(scope, &id)

	awsprovider.NewAwsProvider(stack, jsii.String("AWS"), &awsprovider.AwsProviderConfig{
		Region: jsii.String("us-east-1"),
	})

	bucket := s3bucket.NewS3Bucket(stack, jsii.String("bucket"), &s3bucket.S3BucketConfig{
		Bucket: jsii.String("ilhicas-cdtkf-aws-lambda"),
	})
	cwd, _ := os.Getwd()

	lambdaFile := cdktf.NewTerraformAsset(stack, jsii.String("lambda-file"), &cdktf.TerraformAssetConfig{
		Path: jsii.String(path.Join(cwd, "lambda")),
		Type: cdktf.AssetType_ARCHIVE,
	})

	lambdaS3Object := s3bucketobject.NewS3BucketObject(stack, jsii.String("lambda-archive"), &s3bucketobject.S3BucketObjectConfig{
		Bucket: bucket.Bucket(),
		Key:    lambdaFile.FileName(),
		Source: lambdaFile.Path(),
	})

	lambdaRolePolicy := `
	{
		"Version": "2012-10-17",
		"Statement": [
		  {
			"Action": "sts:AssumeRole",
			"Principal": {
			  "Service": "lambda.amazonaws.com"
			},
			"Effect": "Allow",
			"Sid": ""
		  }
		]
	}`

	lambdaRole := iamrole.NewIamRole(stack, jsii.String("cdktf-lambda-role"), &iamrole.IamRoleConfig{
		AssumeRolePolicy: &lambdaRolePolicy,
	})
	functionName := "cdktf-aws-go-lambda"
	runtime := "go1.x"
	handler := "main"
	path := cdktf.Token_AsString(cdktf.Fn_Abspath(lambdaFile.Path()), &cdktf.EncodingOptions{})
	hash := cdktf.Fn_Filebase64sha256(path)
	lambda := lambdafunction.NewLambdaFunction(stack, jsii.String(functionName), &lambdafunction.LambdaFunctionConfig{
		FunctionName:   &functionName,
		S3Bucket:       bucket.Bucket(),
		S3Key:          lambdaS3Object.Key(),
		Role:           lambdaRole.Arn(),
		Runtime:        &runtime,
		Handler:        &handler,
		SourceCodeHash: hash,
	})
	lambda.Arn()
	return stack
}

func main() {
	app := cdktf.NewApp(nil)

	AWSLambda(app, "cdktf-aws-lambda")

	app.Synth()
}
