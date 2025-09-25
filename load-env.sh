#!/bin/bash

# Load environment variables from .env file
# Usage: source ./load-env.sh

ENV_FILE=".env"

if [ -f "$ENV_FILE" ]; then
    echo "Loading environment variables from $ENV_FILE..."
    
    # Export variables from .env file
    set -a  # automatically export all variables
    source "$ENV_FILE"
    set +a  # stop automatically exporting
    
    echo "Environment variables loaded successfully!"
    echo "Available IARNet environment variables:"
    env | grep "^IARNET_\|^IGNIS_" | sort
else
    echo "Error: $ENV_FILE file not found!"
    echo "Please copy .env.example to .env and configure your settings."
    exit 1
fi