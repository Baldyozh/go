# Kafka Log Processor

A clean architecture Go application that reads logs from Kafka, encrypts sensitive fields using HashiCorp Vault, and processes them.

## Architecture

This project follows **Clean Architecture** principles with clear separation of concerns:

```
├── cmd/                           # Application entry points
│   ├── processor/                 # Main log processor binary
│   └── addlogs/                   # Test utility to add logs
├── internal/                      # Private application code
│   ├── domain/                    # Enterprise business entities
│   │   └── entities/              # Log, ClickHouseLogRecord
│   ├── usecase/                   # Business logic/use cases
│   │   ├── process_logs/          # Log processing use case
│   │   └── add_logs/              # Log addition use case
│   └── infrastructure/            # External concerns implementations
│       ├── kafka/                 # Kafka consumer/producer
│       ├── vault/                 # Vault encryption manager
│       └── config/                # Configuration loading
└── config/                        # Configuration files
```

### Architecture Layers

1. **Domain Layer** (`internal/domain/`)
   - Enterprise business entities (Log, ClickHouseLogRecord)
   - No dependencies on external libraries
   - Core business rules

2. **Use Case Layer** (`internal/usecase/`)
   - Application business logic
   - Defines interfaces it needs (LogConsumer, LogEncrypter, LogProducer)
   - Independent of infrastructure implementation details
   - Orchestrates domain entities to fulfill business requirements

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - Implements interfaces defined by use cases
   - External framework adapters (Kafka, Vault, Config)
   - Can depend on external libraries

4. **Entry Points** (`cmd/`)
   - Dependency injection and composition root
   - Wires infrastructure implementations to use cases
   - Minimal code, delegates to use cases

## Building and Running

### Prerequisites

- Go 1.25.5 or later
- Docker and Docker Compose (for infrastructure services)

### Build

```bash
# Build all binaries
go build -o bin/processor ./cmd/processor
go build -o bin/addlogs ./cmd/addlogs

# Or build both at once
go build -v ./cmd/...
```

### Run Infrastructure

```bash
# Start Kafka, Vault, and ClickHouse
docker-compose up -d
```

### Run Applications

```bash
# Add test messages to Kafka
./bin/addlogs

# Start the log processor
./bin/processor

# Or with custom config path
./bin/processor config/config.yaml
```

### Run Tests

```bash
# Run all tests
go test -v ./internal/...

# Run tests for specific package
go test -v ./internal/infrastructure/vault
```

## Key Features

### 1. Clean Architecture Benefits

- **Testability**: Use cases depend on abstractions, making them easy to test with mocks
- **Maintainability**: Clear boundaries between layers, easy to locate code
- **Framework Independence**: Infrastructure can be swapped without affecting business logic
- **Dependency Rule**: Source code dependencies always point inward toward use cases

### 2. Vault Integration

- **AES-256-GCM Encryption**: Industry-standard encryption
- **Automatic Key Rotation**: Keys rotate every 24 hours
- **Field-Level Encryption**: Encrypt specific JSON fields selectively
- **Positional Path Support**: Encrypt nested fields like `user.profile.contact.email`
- **Wildcard Support**: Encrypt arrays with `items.*.secret`

### 3. Kafka Integration

- **Consumer Group Support**: Multiple processor instances with load balancing
- **Graceful Shutdown**: Handles SIGINT/SIGTERM signals
- **Error Handling**: Continues processing on individual message failures

## Configuration

### config/config.yaml

```yaml
kafka:
  brokers:
    - "localhost:9092"
  topic: "logs"
  group_id: "log-processor"
crypto:
  fields_to_encrypt:
    - "passport"
    - "snils"
    - "password"
    - "api_key"
vault:
  address: "http://localhost:8200"
  token: "my-root-token"
  transit_path: "transit"
  key_name: "kafka-encryption"
```

## Encryption Patterns

The encryptor supports flexible field selection:

### Global Names
Encrypts all fields with matching names regardless of depth:
```go
fields := []string{"password", "api_key"}
// Encrypts: {"password": "..."}, {"user": {"password": "..."}}, etc.
```

### Positional Paths
Encrypts specific paths:
```go
fields := []string{"user.email", "companies.*.secret"}
// Encrypts only those exact paths
```

### Wildcards
Encrypt all array elements:
```go
fields := []string{"items.*.secret"}
// Encrypts: items[0].secret, items[1].secret, etc.
```

## Development

### Adding New Features

1. **New Infrastructure Component**: Add to `internal/infrastructure/`
2. **New Use Case**: Create in `internal/usecase/` with interfaces
3. **New Entity**: Add to `internal/domain/entities/`
4. **Wire It Together**: Update `cmd/` entry point

### Testing Strategy

- **Domain Layer**: No tests needed (simple data structures)
- **Use Case Layer**: Test with mocked infrastructure interfaces
- **Infrastructure Layer**: Integration tests with real services
- **Entry Points**: Manual testing only (thin composition layer)

## Project Structure Rationale

### Why Clean Architecture?

1. **Separation of Concerns**: Business logic is isolated from frameworks
2. **Testability**: Use cases can be tested without Kafka/Vault running
3. **Flexibility**: Easy to swap Kafka for another message broker
4. **Maintainability**: New developers can quickly understand code organization
5. **Scalability**: Easy to add new features without creating spaghetti code

### Interface Placement

Interfaces are defined in the **usecase layer** where they're needed, not where they're implemented. This ensures:
- Use cases control the interface contract
- Infrastructure adapts to use case needs
- No dependency on external libraries in business logic

## License

MIT
