#!/bin/sh

echo "Waiting for MinIO to start..."
sleep 5

echo "Creating bucket: volta-secrets"
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/volta-secrets --ignore-existing

echo "Setting bucket policy"
mc anonymous set download local/volta-secrets

echo "MinIO initialization complete"