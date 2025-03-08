#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
  export $(cat .env | xargs)
fi

echo "🚀 Building Docker image..."
docker build -t prception .

echo "🛑 Stopping existing container..."
docker stop prception || true

docker rm prception || true

echo "🚀 Starting container..."
docker run -d --name prception -p 8081:8080 --env-file .env prception


echo "📜 Logs:"
docker logs -f prception
