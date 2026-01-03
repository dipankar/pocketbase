#!/bin/bash
# Initialize S3 bucket for PocketBase tenants

echo "Creating S3 bucket for PocketBase..."
awslocal s3 mb s3://pocketbase-tenants
awslocal s3api put-bucket-versioning \
    --bucket pocketbase-tenants \
    --versioning-configuration Status=Enabled
echo "S3 bucket created successfully"
