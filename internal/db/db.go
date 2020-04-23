package db

import (
	"dynamodb-golang-sample/internal/config"
	"dynamodb-golang-sample/internal/log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// NewDatabase is initiate the SQL database
func NewDatabase(cfg config.DynamoConfig) error {
	// Create database
	log.I("start newsession...")
	sess, sessErr := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
		Retryer: client.DefaultRetryer{
			NumMaxRetries:    2,
			MinRetryDelay:    0,
			MinThrottleDelay: 0,
			MaxRetryDelay:    60 * time.Second,
			MaxThrottleDelay: 0,
		},
	})
	if sessErr != nil {
		log.E("%v", sessErr.Error())
		return sessErr
	}

	svc := dynamodb.New(sess)

	// Create table Movies
	tableName := "Movies"
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("Year"),
				AttributeType: aws.String("N"),
			},
			{
				AttributeName: aws.String("Title"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("Year"),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String("Title"),
				KeyType:       aws.String("RANGE"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(tableName),
	}

	_, err := svc.CreateTable(input)
	if err != nil {
		log.E("Got error calling CreateTable: %v", err.Error())
		return err
	}

	log.I("Created the table %v", tableName)

	log.I("Successfully connected to Dynamo database: %v", cfg.Endpoint)

	return nil
}
