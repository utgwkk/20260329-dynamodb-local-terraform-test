terraform {
  required_providers {
    aws = "~> 6.0"
  }
  backend "local" {
    path = "terraform.tfstate"
  }
}

provider "aws" {
  region = "ap-northeast-1"

  access_key = "DUMMY"
  secret_key = "DUMMY"

  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    dynamodb = "http://localhost:8000"
  }
}

resource "aws_dynamodb_table" "test" {
  name         = "test"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "pk"
    type = "S"
  }
  attribute {
    name = "sk"
    type = "S"
  }

  hash_key  = "pk"
  range_key = "sk"
}
