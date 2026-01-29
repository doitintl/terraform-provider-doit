#!/bin/bash
# Test script to check which GET and DELETE endpoints return 404 vs 500 for non-existent IDs
# Related to: https://doitintl.atlassian.net/browse/CMP-37040

set -e

# Configuration
API_HOST="${DOIT_HOST:-api.doit.com}"
API_TOKEN="${DOIT_API_TOKEN}"
CUSTOMER_CONTEXT="${DOIT_CUSTOMER_CONTEXT}"

if [ -z "$API_TOKEN" ]; then
    echo "Error: DOIT_API_TOKEN environment variable is not set"
    exit 1
fi

if [ -z "$CUSTOMER_CONTEXT" ]; then
    echo "Error: DOIT_CUSTOMER_CONTEXT environment variable is not set"
    exit 1
fi

# Use a UUID-like fake ID that definitely won't exist
FAKE_ID="00000000-0000-0000-0000-000000000000"

# Function to test an endpoint
test_endpoint() {
    local method="$1"
    local endpoint="$2"

    # Make the request and capture status code and body
    response=$(curl -s -w "\n%{http_code}" \
        -X "$method" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Accept: application/json" \
        "${API_HOST}${endpoint}?customerContext=${CUSTOMER_CONTEXT}" 2>/dev/null)

    # Split response into body and status code
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d' | head -c 60 | tr '\n' ' ')

    # Determine display
    if [ "$status_code" = "404" ] || [ "$status_code" = "204" ]; then
        status_display="✓ $status_code"
    elif [ "$status_code" = "500" ]; then
        status_display="✗ 500"
    else
        status_display="? $status_code"
    fi

    printf "| %-7s | %-50s | %-7s | %s |\n" "$method" "$endpoint" "$status_display" "$body..."
}

echo "=========================================="
echo "API 404 Response Testing (GET and DELETE)"
echo "Host: ${API_HOST}"
echo "Customer Context: ${CUSTOMER_CONTEXT}"
echo "Fake ID: ${FAKE_ID}"
echo "=========================================="
echo ""
echo "| Method  | Endpoint                                           | Status  | Response (truncated) |"
echo "|---------|----------------------------------------------------|---------|-----------------------|"

# Test GET endpoints
test_endpoint "GET" "/analytics/v1/alerts/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/allocations/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/annotations/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/labels/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/attributiongroups/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/attributions/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/budgets/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/reports/${FAKE_ID}"
test_endpoint "GET" "/analytics/v1/reports/${FAKE_ID}/config"
test_endpoint "GET" "/billing/v1/invoices/${FAKE_ID}"
test_endpoint "GET" "/core/v1/cloudincidents/${FAKE_ID}"
test_endpoint "GET" "/iam/v1/users/${FAKE_ID}"
test_endpoint "GET" "/billing/v1/assets/${FAKE_ID}"

echo ""
echo "| Method  | Endpoint                                           | Status  | Response (truncated) |"
echo "|---------|----------------------------------------------------|---------|-----------------------|"

# Test DELETE endpoints
test_endpoint "DELETE" "/analytics/v1/alerts/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/allocations/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/annotations/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/labels/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/attributiongroups/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/attributions/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/budgets/${FAKE_ID}"
test_endpoint "DELETE" "/analytics/v1/reports/${FAKE_ID}"
test_endpoint "DELETE" "/iam/v1/users/${FAKE_ID}"

echo ""
echo "=========================================="
echo "Legend: ✓ = Correct (404/204), ✗ = Incorrect (500)"
echo "=========================================="
