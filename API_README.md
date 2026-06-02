# Log Processor API Documentation

## Overview

This API provides endpoints for managing and querying encrypted logs stored in ClickHouse with role-based access control and Vault integration for sensitive data decryption.

## Authentication

The API uses JWT tokens for authentication. Include the token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### Default Admin User

- **Username**: `admin`
- **Password**: `admin123`
- **Role**: `admin` (full access)

## Roles and Permissions

### Integration Developer
- `logs:read` - Read logs
- `logs:search` - Search logs by request_id
- `logs:filter` - Filter logs
- `logs:stats` - View log statistics

### Incident Analyst
- `logs:read` - Read logs
- `logs:search` - Search logs by request_id
- `logs:filter` - Filter logs
- `logs:decrypt` - Decrypt sensitive fields
- `logs:stats` - View log statistics

### Admin
- All permissions including `users:manage` for user management

## API Endpoints

### Authentication

#### POST /api/auth/login
Login and receive JWT token.

**Request Body:**
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "username": "admin",
    "email": "admin@example.com",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "roles": [...]
  }
}
```

#### GET /api/auth/me
Get current authenticated user info.

**Headers:** `Authorization: Bearer <token>`

**Response:**
```json
{
  "id": 1,
  "username": "admin",
  "email": "admin@example.com",
  "roles": [...]
}
```

#### GET /api/auth/roles
Get all available roles.

**Headers:** `Authorization: Bearer <token>`

**Response:**
```json
[
  {
    "id": 1,
    "name": "integration_developer",
    "description": "Разработчик интеграций"
  },
  ...
]
```

### Logs

#### GET /api/logs
Query logs with filters.

**Headers:** `Authorization: Bearer <token>`

**Query Parameters:**
- `request_id` (string) - Filter by request ID
- `integration_id` (string) - Filter by integration ID
- `start_time` (string, RFC3339) - Start timestamp
- `end_time` (string, RFC3339) - End timestamp
- `status_code` (number) - Filter by HTTP status code
- `http_method` (string) - Filter by HTTP method
- `endpoint` (string) - Filter by endpoint (partial match)
- `user_id` (number) - Filter by user ID
- `success` (boolean) - Filter by success status

**Example:**
```
GET /api/logs?integration_id=partner1&start_time=2024-01-01T00:00:00Z&status_code=200
```

**Response:**
```json
[
  {
    "log_id": "uuid",
    "timestamp": "2024-01-01T00:00:00Z",
    "integration_id": "partner1",
    "request_id": "req-123",
    "http_method": "POST",
    "endpoint": "/api/v1/users",
    "request_body": "{\"encrypted\": true, ...}",
    "response_body": "{\"encrypted\": true, ...}",
    "duration_ms": 150,
    "status_code": 200,
    "success": true,
    "error_message": "",
    "user_id": 123
  }
]
```

#### GET /api/logs/{id}
Get a specific log by ID.

**Headers:** `Authorization: Bearer <token>`

**Query Parameters:**
- `decrypt` (boolean) - Decrypt sensitive fields (requires `logs:decrypt` permission)
- `reason` (string) - Reason for decryption request (logged in PostgreSQL)

**Example:**
```
GET /api/logs/uuid-123?decrypt=true&reason=Incident%20investigation%20%231234
```

**Response:**
```json
{
  "log_id": "uuid-123",
  "timestamp": "2024-01-01T00:00:00Z",
  "integration_id": "partner1",
  "request_id": "req-123",
  "http_method": "POST",
  "endpoint": "/api/v1/users",
  "request_body": "{\"passport\": \"1234567890\", ...}",
  "response_body": "{\"data\": ...}",
  "duration_ms": 150,
  "status_code": 200,
  "success": true,
  "error_message": "",
  "user_id": 123
}
```

#### GET /api/logs/request/{request_id}
Get all logs for a specific request ID.

**Headers:** `Authorization: Bearer <token>`

**Example:**
```
GET /api/logs/request/req-123
```

**Response:** Same as `/api/logs` endpoint

#### GET /api/logs/stats
Get log statistics.

**Headers:** `Authorization: Bearer <token>`

**Query Parameters:** Same as `/api/logs` (for filtering)

**Response:**
```json
{
  "total_logs": 10000,
  "successful_logs": 9500,
  "failed_logs": 500,
  "avg_duration_ms": 150.5,
  "p95_duration_ms": 300.0
}
```

#### GET /api/logs/export
Export logs to CSV format.

**Headers:** `Authorization: Bearer <token>`

**Query Parameters:** Same as `/api/logs`

**Response:** CSV file download

### Admin

#### POST /api/admin/users
Create a new user (requires `users:manage` permission).

**Headers:** `Authorization: Bearer <token>`

**Request Body:**
```json
{
  "username": "newuser",
  "email": "newuser@example.com",
  "password": "securepassword",
  "role_ids": [1, 2]
}
```

**Response:**
```json
{
  "id": 2,
  "username": "newuser",
  "email": "newuser@example.com",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "roles": [...]
}
```

## Running the API

### Start the API Server

```bash
go run cmd/api/main.go
```

Or with custom config:

```bash
go run cmd/api/main.go config/config.yaml
```

### Running with Docker Compose

Add to `docker-compose.yml`:

```yaml
services:
  api:
    build: .
    command: go run cmd/api/main.go
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - clickhouse
      - vault
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      CLICKHOUSE_ADDRESS: clickhouse
      VAULT_ADDRESS: http://vault:8200
    networks:
      - postgres
```

Then run:

```bash
docker compose up -d
```

## Configuration

Edit `config/config.yaml`:

```yaml
postgres:
  host: "localhost"
  port: 5432
  user: "pguser"
  password: "pgpassword"
  database: "log-processor"

api:
  port: "8080"
  jwt_secret: "your-secret-key-change-in-production"
  jwt_expiry: "24h"
  jwt_issuer: "log-processor-api"
```

## Database Migrations

Run migrations manually:

```bash
docker exec postgres_container1 psql -U pguser -d log-processor -f /dev/stdin < migrations/001_init_auth.sql
docker exec postgres_container1 psql -U pguser -d log-processor -f /dev/stdin < migrations/002_create_admin_user.sql
```

## Security Notes

1. **Change the default admin password** immediately after first login
2. **Update `jwt_secret`** in production with a strong random key
3. **Use HTTPS** in production environments
4. **Review decryption logs** regularly in the `decryption_requests` table
5. **Limit JWT expiry** for sensitive operations

## Sensitive Fields

The following fields are encrypted by Vault (configurable in `config.yaml`):
- `passport`
- `snils`

Decryption requires the `logs:decrypt` permission and is logged for audit purposes.
