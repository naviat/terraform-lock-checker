package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/spf13/cobra"
)

type DynamoDBLock struct {
	LockID    string `json:"LockID"`
	Info      string `json:"Info"`
	Operation string `json:"Operation"`
}

var rootCmd = &cobra.Command{
	Use:   "terraform-lock-checker",
	Short: "Terraform/Terragrunt Lock Checker",
	Run: func(cmd *cobra.Command, args []string) {
		promptForCloudProvider()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func promptForCloudProvider() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Which cloud provider are you using? (aws/azure): ")
	cloudProvider, _ := reader.ReadString('\n')
	cloudProvider = strings.TrimSpace(cloudProvider)

	switch strings.ToLower(cloudProvider) {
	case "aws":
		promptForAWSDetails()
	case "azure":
		promptForAzureDetails()
	default:
		fmt.Println("Invalid cloud provider specified.")
	}
}

func promptForAWSDetails() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Please set AWS_REGION environment variable: ")
	awsRegion, _ := reader.ReadString('\n')
	os.Setenv("AWS_REGION", strings.TrimSpace(awsRegion))

	fmt.Print("Enter the DynamoDB table name for locking: ")
	tableName, _ := reader.ReadString('\n')
	tableName = strings.TrimSpace(tableName)

	handleDynamoDBLocks(tableName)
}

func promptForAzureDetails() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Please set AZURE_STORAGE_ACCOUNT_NAME environment variable: ")
	azureAccountName, _ := reader.ReadString('\n')
	os.Setenv("AZURE_STORAGE_ACCOUNT_NAME", strings.TrimSpace(azureAccountName))

	fmt.Print("Please set AZURE_STORAGE_ACCOUNT_KEY environment variable: ")
	azureAccountKey, _ := reader.ReadString('\n')
	os.Setenv("AZURE_STORAGE_ACCOUNT_KEY", strings.TrimSpace(azureAccountKey))

	fmt.Print("Enter the Azure Blob container name for locking: ")
	containerName, _ := reader.ReadString('\n')
	containerName = strings.TrimSpace(containerName)

	handleAzureBlobLocks(containerName)
}

func handleDynamoDBLocks(tableName string) {
	region := os.Getenv("AWS_REGION")
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		log.Fatalf("Failed to connect to AWS: %v", err)
	}

	svc := dynamodb.New(sess)

	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Fatalf("Failed to scan DynamoDB table: %v", err)
	}

	var locks []DynamoDBLock
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &locks)
	if err != nil {
		log.Fatalf("Failed to unmarshal scan result: %v", err)
	}

	if len(locks) == 0 {
		fmt.Println("No locks found in DynamoDB table.")
	} else {
		fmt.Println("Current locks in DynamoDB table:")
		for _, lock := range locks {
			fmt.Printf("LockID: %s, Info: %s, Operation: %s\n", lock.LockID, lock.Info, lock.Operation)
			promptForUnlockDynamoDB(svc, tableName, lock.LockID)
		}
	}
}

func promptForUnlockDynamoDB(svc *dynamodb.DynamoDB, tableName, lockID string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Do you want to unlock LockID %s? (y/n): ", lockID)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if response == "y" || response == "Y" {
		_, err := svc.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key: map[string]*dynamodb.AttributeValue{
				"LockID": {
					S: aws.String(lockID),
				},
			},
		})
		if err != nil {
			log.Printf("Failed to delete lock with LockID %s: %v", lockID, err)
		} else {
			fmt.Printf("Deleted lock with LockID %s\n", lockID)
		}
	}
}

func handleAzureBlobLocks(containerName string) {
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT_NAME")
	accountKey := os.Getenv("AZURE_STORAGE_ACCOUNT_KEY")

	// Create a credentials object with your Azure Storage Account name and key.
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatalf("Failed to create Azure credential: %v", err)
	}

	// From the Azure portal, get your Storage account blob service URL endpoint.
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)

	// Create a client object that wraps the service URL and a request pipeline to make requests.
	client, err := azblob.NewServiceClientWithSharedKeyCredential(serviceURL, credential, nil)
	if err != nil {
		log.Fatalf("Failed to create Azure service client: %v", err)
	}

	containerClient := client.NewContainerClient(containerName)
	pager := containerClient.NewListBlobsFlatPager(&azblob.ListBlobsFlatOptions{})

	var locks []azblob.BlobItem
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			log.Fatalf("Failed to list blobs: %v", err)
		}
		locks = append(locks, resp.Segment.BlobItems...)
	}

	if len(locks) == 0 {
		fmt.Println("No locks found in Azure Blob container.")
	} else {
		fmt.Println("Current locks in Azure Blob container:")
		for _, blob := range locks {
			fmt.Printf("BlobName: %s\n", *blob.Name)
			promptForUnlockAzureBlob(containerClient, *blob.Name)
		}
	}
}

func promptForUnlockAzureBlob(containerClient *azblob.ContainerClient, blobName string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Do you want to unlock Blob %s? (y/n): ", blobName)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if response == "y" || response == "Y" {
		blobClient := containerClient.NewBlobClient(blobName)
		_, err := blobClient.Delete(context.Background(), nil)
		if err != nil {
			log.Printf("Failed to delete blob %s: %v", blobName, err)
		} else {
			fmt.Printf("Deleted blob %s\n", blobName)
		}
	}
}
