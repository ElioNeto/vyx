#!/bin/bash
# LocalStack initialization script
# Creates test resources for integration tests

set -e

echo "Creating DynamoDB lock table..."
awslocal dynamodb create-table \
  --table-name vyx-infra-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1

echo "Creating S3 state bucket..."
awslocal s3 mb s3://vyx-test-state --region us-east-1
awslocal s3api put-bucket-versioning \
  --bucket vyx-test-state \
  --versioning-configuration Status=Enabled

echo "LocalStack initialized successfully."
