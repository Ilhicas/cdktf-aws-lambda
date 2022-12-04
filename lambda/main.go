package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type Item struct {
	RequestId string `json:"RequestId"`
	Value     string `json:"Value"`
	Error     string `json:"Error"`
}

type MyEvent struct {
	Name string `json:"name"`
}

type APIGatewayResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IsBase64Encoded bool              `json:"isBase64Encoded,omitempty"`
}

type Response struct {
	Message Item   `json:"message"`
	Level   string `json:"level"`
}

func handleResponse(status int, msg Item, level string) (APIGatewayResponse, error) {
	body, _ := json.Marshal(Response{
		Message: msg,
		Level:   level,
	})

	return APIGatewayResponse{
		StatusCode: status,
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body:            string(body),
		IsBase64Encoded: false,
	}, nil
}

func HandleRequest(ctx context.Context, name MyEvent) (APIGatewayResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)
	tableName := aws.String(os.Getenv("TABLE_NAME"))
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	out := Item{}
	db := dynamodb.New(session)
	item, err := dynamodbattribute.MarshalMap(&Item{
		RequestId: lc.AwsRequestID,
		Value:     lc.InvokedFunctionArn,
	})
	if err != nil {
		out.Error = "Invalid Request"
		return handleResponse(400, out, "ERROR")
	}
	input := &dynamodb.PutItemInput{
		Item:      item,
		TableName: tableName,
	}

	_, err = db.PutItem(input)

	if err != nil {
		out.Error = "Unable to write to the Database"
		return handleResponse(500, out, "ERROR")
	}
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: tableName,
		Key: map[string]*dynamodb.AttributeValue{
			"RequestId": {
				S: aws.String(lc.AwsRequestID),
			},
			"Value": {
				S: aws.String(lc.InvokedFunctionArn),
			},
		},
	})
	if err != nil {
		out.Error = "Unable to read from the Database"
		return handleResponse(500, out, "ERROR")
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &out)
	if err != nil {
		out.Error = "Unable to Unmarshall item"
		return handleResponse(500, out, "ERROR")
	}
	return handleResponse(200, out, "INFO")
}

func main() {
	lambda.Start(HandleRequest)
}
