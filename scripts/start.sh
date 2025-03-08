#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
  export $(cat .env | xargs)
fi

echo "🚀 Building Docker image..."
docker build -t auto-pr-approver .

echo "🛑 Stopping existing container..."
docker stop auto-pr-approver || true

docker rm auto-pr-approver || true

echo "🚀 Starting container..."
docker run -d --name auto-pr-approver -p 8081:8080 --env-file .env auto-pr-approver


echo "📜 Logs:"
docker logs -f auto-pr-approver
