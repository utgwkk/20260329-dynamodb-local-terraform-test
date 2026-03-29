#!/bin/sh
set -ex
rm terraform.tfstate
aws --profile=dynamodblocal --endpoint-url=http://localhost:8000/ dynamodb delete-table --table-name test
