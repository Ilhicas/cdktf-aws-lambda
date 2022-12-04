package main

import (
	"fmt"
	"os"
	"path"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewaydeployment"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewayintegration"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewaymethod"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewayresource"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewayrestapi"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/apigatewaystage"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/dataawscalleridentity"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/dynamodbtable"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/iampolicy"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/iamrole"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/iamrolepolicyattachment"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/lambdafunction"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/lambdapermission"
	awsprovider "github.com/cdktf/cdktf-provider-aws-go/aws/v10/provider"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/s3bucket"
	"github.com/cdktf/cdktf-provider-aws-go/aws/v10/s3bucketobject"
	"github.com/hashicorp/terraform-cdk-go/cdktf"
)

type IamInlinePolicyStruct struct {
	Version  string   `json:"Version"`
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource string   `json:"Resource"`
}

func AWSLambdaStack(scope constructs.Construct, id string) cdktf.TerraformStack {
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
		Environment: &lambdafunction.LambdaFunctionEnvironment{
			Variables: &map[string]*string{*jsii.String("TABLE_NAME"): jsii.String("cdktf-ilhicas-dynamodb")},
		},
	})

	apiGwRestApi := apigatewayrestapi.NewApiGatewayRestApi(stack, jsii.String("cdktf-api-gw-rest-api"), &apigatewayrestapi.ApiGatewayRestApiConfig{
		Name: jsii.String("ilhicas-cdktf-api-gw"),
	})

	apiGwResource := apigatewayresource.NewApiGatewayResource(stack, jsii.String("cdktf-api-gw-resource"), &apigatewayresource.ApiGatewayResourceConfig{
		PathPart:  jsii.String("resource"),
		ParentId:  apiGwRestApi.RootResourceId(),
		RestApiId: apiGwRestApi.Id(),
	})

	apiGwMethod := apigatewaymethod.NewApiGatewayMethod(stack, jsii.String("cdktf-api-gw-method"), &apigatewaymethod.ApiGatewayMethodConfig{
		RestApiId:     apiGwRestApi.Id(),
		ResourceId:    apiGwResource.Id(),
		HttpMethod:    jsii.String("GET"),
		Authorization: jsii.String("NONE"),
	})

	apiGwIntegration := apigatewayintegration.NewApiGatewayIntegration(stack, jsii.String("cdktf-api-gw-lambda-integration"), &apigatewayintegration.ApiGatewayIntegrationConfig{
		RestApiId:             apiGwRestApi.Id(),
		ResourceId:            apiGwResource.Id(),
		HttpMethod:            apiGwMethod.HttpMethod(),
		IntegrationHttpMethod: jsii.String("POST"),
		Type:                  jsii.String("AWS_PROXY"),
		Uri:                   lambda.InvokeArn(),
	})
	apiGwIntegration.Id()
	callerIdentity := dataawscalleridentity.NewDataAwsCallerIdentity(stack, jsii.String("current"), &dataawscalleridentity.DataAwsCallerIdentityConfig{})

	// source_arn = "arn:aws:execute-api:${var.myregion}:${var.accountId}:${aws_api_gateway_rest_api.api.id}/*/${aws_api_gateway_method.method.http_method}${aws_api_gateway_resource.resource.path}"
	lambdaPermissionSourceArn := jsii.String(fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/*/%s%s",
		*jsii.String("us-east-1"),
		*callerIdentity.AccountId(),
		*cdktf.Token_AsString(apiGwRestApi.Id(), &cdktf.EncodingOptions{}),
		*cdktf.Token_AsString(apiGwMethod.HttpMethod(), &cdktf.EncodingOptions{}),
		*cdktf.Token_AsString(apiGwResource.Path(), &cdktf.EncodingOptions{}),
	),
	)

	lambdapermission.NewLambdaPermission(stack, jsii.String("cktf-api-gw-lambda-permission"), &lambdapermission.LambdaPermissionConfig{
		StatementId:  jsii.String("AllowExecutionFromAPIGateway"),
		Action:       jsii.String("lambda:InvokeFunction"),
		FunctionName: lambda.FunctionName(),
		Principal:    jsii.String("apigateway.amazonaws.com"),
		SourceArn:    lambdaPermissionSourceArn,
	})

	referenceableListTrigger := []*string{cdktf.Token_AsString(apiGwIntegration.Id(), &cdktf.EncodingOptions{}),
		cdktf.Token_AsString(apiGwMethod.Id(), &cdktf.EncodingOptions{}),
		cdktf.Token_AsString(apiGwResource.Id(), &cdktf.EncodingOptions{}),
	}
	conditionTrigger := &map[string]*string{
		"redeployment": cdktf.Fn_Sha1(cdktf.Fn_Jsonencode(referenceableListTrigger)),
	}

	apiGwDeployment := apigatewaydeployment.NewApiGatewayDeployment(stack, jsii.String("cdktf-api-gw-deployment"), &apigatewaydeployment.ApiGatewayDeploymentConfig{
		RestApiId: apiGwRestApi.Id(),
		Triggers:  conditionTrigger,
	})

	apiGwStage := apigatewaystage.NewApiGatewayStage(stack, jsii.String("cdktf-api-gw-stage-prd"), &apigatewaystage.ApiGatewayStageConfig{
		DeploymentId: apiGwDeployment.Id(),
		RestApiId:    apiGwRestApi.Id(),
		StageName:    jsii.String("prd"),
	})

	dynamoTable := dynamodbtable.NewDynamodbTable(stack, jsii.String("cdktf-dynamodb-table"), &dynamodbtable.DynamodbTableConfig{
		Name:        jsii.String("cdktf-ilhicas-dynamodb"),
		BillingMode: jsii.String("PAY_PER_REQUEST"),
		HashKey:     jsii.String("RequestId"),
		RangeKey:    jsii.String("Value"),
		Attribute: &[]dynamodbtable.DynamodbTableAttribute{
			{Name: jsii.String("RequestId"), Type: jsii.String("S")},
			{Name: jsii.String("Value"), Type: jsii.String("S")},
		},
	})

	policyRw := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "DynamoDBTableAccess",
				"Effect": "Allow",
				"Action": [
					"dynamodb:BatchGetItem",
					"dynamodb:BatchWriteItem",
					"dynamodb:ConditionCheckItem",
					"dynamodb:PutItem",
					"dynamodb:DescribeTable",
					"dynamodb:DeleteItem",
					"dynamodb:GetItem",
					"dynamodb:Scan",
					"dynamodb:Query",
					"dynamodb:UpdateItem"
				],
				"Resource": "%s"
			}
		]
	}`, *dynamoTable.Arn())

	rwDynamodbPolicy := iampolicy.NewIamPolicy(stack, jsii.String("cdktf-dynamodb-policy-rw"), &iampolicy.IamPolicyConfig{
		Name:        jsii.String("cdktf-dynamodb-policy-rw"),
		Description: jsii.String("Read And Write Permissions for CDKTF DynamodbTable"),
		Policy:      jsii.String(policyRw),
	})

	iamrolepolicyattachment.NewIamRolePolicyAttachment(stack, jsii.String("cdktf-lambda-dynamodb-policy-attachment"), &iamrolepolicyattachment.IamRolePolicyAttachmentConfig{
		Role:      lambdaRole.Name(),
		PolicyArn: rwDynamodbPolicy.Arn(),
	})

	cdktf.NewTerraformOutput(stack, jsii.String("lamda-endpoint"), &cdktf.TerraformOutputConfig{
		Value: jsii.String(fmt.Sprintf("%s%s", *apiGwStage.InvokeUrl(), *apiGwResource.Path())),
	})

	return stack
}

func main() {
	app := cdktf.NewApp(nil)

	AWSLambdaStack(app, "cdktf-aws-lambda")

	app.Synth()
}
