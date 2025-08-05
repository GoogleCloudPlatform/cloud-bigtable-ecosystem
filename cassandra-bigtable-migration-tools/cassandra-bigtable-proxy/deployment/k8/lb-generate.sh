#!/bin/bash

# This script reads ports from config file and populates ports dynamically in k8s load balancer service.

# Config file path
CONFIG_FILE="config.yaml"

# Base templates
SERVICE_TEMPLATE="loadbalancer-service.yaml"

# Extract ports
PORTS=$(grep 'port:' $CONFIG_FILE | awk '{print $2}')

# Prepare the ports entries for the Deployment
PORT_ENTRIES=""
for port in $PORTS; do
    PORT_ENTRIES="${PORT_ENTRIES}  - port: $port\n    targetPort: $port\n    protocol: TCP\n    name: port-$port\n"
done

# Replace the placeholder in the Deployment template with actual ports
cp $SERVICE_TEMPLATE "loadbalancer-service-dynamic.yaml"
sed -i "s|# Ports will be populated here dynamically|$PORT_ENTRIES|" "loadbalancer-service-dynamic.yaml"

echo "Load balancer manifest file has been generated."
