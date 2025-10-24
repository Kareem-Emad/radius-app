# DNI RADIUS Server Project

A modern, scalable RADIUS server implementation with Redis-based accounting and real-time message streaming capabilities.

## Overview

This project implements a complete RADIUS (Remote Authentication Dial-In User Service) solution with the following components:

- **RADIUS Server**: Handles authentication (port 1812) and accounting (port 1813) requests
- **Redis Backend**: Stores accounting data and manages message streams
- **Stream Consumers**: Real-time processing of accounting updates per user
- **Docker Containerization**: Complete orchestration with Docker Compose

The system is built with Go using clean architecture principles, dependency injection, and interface-based design for maintainability and testability.

## Features

- ✅ RADIUS Authentication and Accounting servers
- ✅ Redis-based data storage with configurable TTL
- ✅ Real-time message streaming using Redis Streams
- ✅ Per-user consumer groups for isolated message processing
- ✅ Comprehensive logging

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for local development)

### Setup and Run

1. **Clone and navigate to the project**:
   ```bash
   cd /path/to/dni
   ```

2. **Start all services**:
   ```bash
   docker-compose up --build -d
   ```

3. **Verify all containers are running**:
   ```bash
   docker-compose ps
   ```

   Expected output:
   ```
   NAME                IMAGE                  COMMAND               STATUS
   radclient-test      alpine:latest          "sh -c '..."          Up (healthy)
   radius-consumer-1   dni-redis-consumer-1   "./redis-consumer"    Up (healthy)
   radius-consumer-2   dni-redis-consumer-2   "./redis-consumer"    Up (healthy)
   radius-redis        redis:7-alpine         "docker-entrypoint"   Up (healthy)
   radius-server       dni-radius-server      "./radius-server"     Up (healthy)
   ```

## Testing and Verification

### Send Test RADIUS Packets

#### Authentication Tests (Port 1812)

1. **Test authentication for testuser-1**:
   ```bash
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/auth_request_1.txt radius-server:1812 auth testing123"
   ```
   Expected response: `Received Access-Accept` (user exists with correct password)

2. **Test authentication for testuser-2**:
   ```bash
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/auth_request_2.txt radius-server:1812 auth testing123"
   ```
   Expected response: `Received Access-Accept` (user exists with correct password)

#### Accounting Tests (Port 1813)

1. **Send accounting start for testuser-1**:
   ```bash
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/acct_start_1.txt radius-server:1813 acct testing123"
   ```

2. **Send accounting start for testuser-2**:
   ```bash
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/acct_start_2.txt radius-server:1813 acct testing123"
   ```

3. **Send accounting stop packets**:
   ```bash
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/acct_stop_1.txt radius-server:1813 acct testing123"
   docker-compose exec radclient-test sh -c "echo 'testing123' | radclient -f /test/acct_stop_2.txt radius-server:1813 acct testing123"
   ```

### Verify System Operation

#### 1. Check Service Logs

**RADIUS Server logs**:
```bash
docker-compose logs radius-server
```
Look for:
- `Connected to Redis at redis:6379`  
- `RADIUS Secret configured: testing123`
- `Starting Authentication server on :1812`
- `Starting Accounting server on :1813`

**Authentication Success Logs**:
- `[AUTH] Received Access-Request from <IP> for user: testuser-1`
- `[AUTH] Access granted for user: testuser-1`

**Authentication Failure Logs**:
- `[AUTH] Access denied for user: <username> (user not found)`
- `[AUTH] Access denied for user: <username> (invalid password)`

**Consumer logs**:
```bash
docker-compose logs redis-consumer-1
docker-compose logs redis-consumer-2
```
Look for:
- `Connected to Redis at redis:6379`
- `Consumer group 'consumer-group-testuser-X' ready`
- `[testuser-X] Received update for key: radius:acct:testuser-X:sessionXXXXX`

#### 2. Check Redis Data

**View all keys**:
```bash
docker-compose exec redis redis-cli keys "*"
```
Expected keys:
- `radius:acct:testuser-1:session12345` (accounting data)
- `radius:acct:testuser-2:session67890` (accounting data)
- `radius:updates:testuser-1` (stream for testuser-1)
- `radius:updates:testuser-2` (stream for testuser-2)

**View accounting data**:
```bash
docker-compose exec redis redis-cli hgetall "radius:acct:testuser-1:session12345"
```

**View stream messages**:
```bash
docker-compose exec redis redis-cli xread STREAMS radius:updates:testuser-1 0
```

**Check stream consumer groups**:
```bash
docker-compose exec redis redis-cli xinfo GROUPS radius:updates:testuser-1
```

#### 3. View Consumer Log Files

```bash
docker-compose exec redis-consumer-1 cat /var/log/radius_updates.log
```

### Expected Test Flow

#### Authentication Flow (Port 1812)
1. **Auth Request**: radclient sends Access-Request to RADIUS server
2. **User Validation**: Server validates username/password against configured credentials
3. **Response**: Server returns Access-Accept (valid user) or Access-Reject (invalid)
4. **Verification**: Check server logs for authentication success/failure messages

#### Accounting Flow (Port 1813)
1. **Packet Sent**: radclient sends accounting packet to RADIUS server
2. **Server Processing**: RADIUS server receives packet, stores data in Redis, publishes to stream
3. **Stream Delivery**: Redis stream delivers message to appropriate consumer group
4. **Consumer Processing**: Consumer receives message, processes it, logs to file
5. **Verification**: Check logs and Redis keys to confirm end-to-end flow

## Project Structure

```
dni/
├── cmd/                    # Application entry points
│   ├── api/               # RADIUS server main application
│   │   ├── main.go        # Server entry point
│   │   └── deps.go        # Dependency initialization
│   └── consumer/          # Redis consumer application
│       ├── main.go        # Consumer entry point
│       └── deps.go        # Consumer dependency initialization
├── internal/              # Private application code
│   ├── accounting/        # Accounting packet handling
│   │   └── handler.go     # RADIUS accounting logic
│   ├── auth/             # Authentication packet handling
│   │   └── handler.go     # RADIUS authentication logic
│   └── consumer/         # Consumer business logic
│       └── consumer.go    # Stream message processing
├── pkg/                  # Public library code
│   ├── config/           # Configuration management
│   │   └── config.go     # Environment variable loading
│   ├── datastore/        # Data storage abstraction
│   │   ├── interface.go   # Datastore interface
│   │   └── redis.go      # Redis implementation
│   └── stream/           # Message streaming abstraction
│       ├── interface.go   # Stream interface
│       └── redis.go      # Redis Streams implementation
├── test/                 # Test files and data
│   ├── auth_request_1.txt # Test authentication for testuser-1
│   ├── auth_request_2.txt # Test authentication for testuser-2
│   ├── acct_start_1.txt  # Test accounting start packets
│   ├── acct_start_2.txt
│   ├── acct_stop_1.txt   # Test accounting stop packets
│   └── acct_stop_2.txt
├── docker-compose.yml    # Container orchestration
├── Dockerfile           # RADIUS server container
├── Dockerfile.consumer  # Consumer container
├── Makefile            # Development commands
└── README.md          # This file
```

## Architecture Overview

### Design Principles

1. **Clean Architecture**: Separation of concerns with clear boundaries between layers
2. **Dependency Injection**: All dependencies are injected rather than created internally
3. **Interface-Based Design**: Core logic depends on interfaces, not concrete implementations
4. **Single Responsibility**: Each component has a focused, well-defined purpose

### Component Responsibilities

#### RADIUS Server (`cmd/api`)
- **Main**: Application bootstrap and server orchestration
- **Deps**: Dependency initialization (Redis, handlers, configuration)
- **Auth Handler**: Processes RADIUS authentication requests
- **Accounting Handler**: Processes RADIUS accounting requests, stores data, publishes streams

#### Consumer (`cmd/consumer`)
- **Main**: Consumer application bootstrap
- **Deps**: Consumer dependency initialization
- **Consumer Logic**: Processes stream messages and writes logs

#### Shared Packages (`pkg/`)
- **Config**: Environment-based configuration management
- **Datastore**: Data persistence abstraction (Redis implementation)
- **Stream**: Message streaming abstraction (Redis Streams implementation)

### Data Flow

1. **RADIUS Request** → Server receives UDP packet
2. **Authentication/Accounting** → Appropriate handler processes request
3. **Data Storage** → Accounting data stored in Redis with TTL
4. **Stream Publishing** → Update message published to user-specific stream
5. **Consumer Processing** → Dedicated consumer processes stream messages
6. **Logging** → Consumer writes processed messages to log files

### Configuration

All services are configured via environment variables:

**Server Configuration**:
- `REDIS_HOST`, `REDIS_PORT`: Redis connection
- `AUTH_PORT`, `ACCT_PORT`: RADIUS server ports
- `RADIUS_SECRET`: RADIUS shared secret (default: "testing123")
- `USER_CREDENTIALS`: User authentication credentials in format "username:password,username:password,..." 
  - Example: "testuser-1:testpass123,testuser-2:testpass456"
- `ACCOUNTING_TTL`: Data retention period

**Consumer Configuration**:
- `REDIS_HOST`, `REDIS_PORT`: Redis connection
- `USERNAME`: User identifier for stream targeting
- `LOG_FILE`: Output log file path

### Docker Services

- **radius-server**: Main RADIUS server (ports 1812/1813)
- **redis**: Redis database and streams (port 6379)
- **redis-consumer-1/2**: Per-user message consumers
- **radclient-test**: Testing container with radclient tools

## Use of AI

### AI Tool Used
This project was developed with the assistance of **GitHub Copilot** (Claude-3.5-Sonnet), an AI-powered coding assistant.

### Usage Methodology
The AI was employed as a **junior developer** in the development process, with a clear focus/drive from me on critical choices that shape the output such as:
  - Interface design and component interactions
  - System architecture and separation of concerns
  - Technology choices and implementation patterns
  - Review and Refining of the code.
  - Code generation based on specified requirements and interfaces

### AI-Generated Deliverables
The following components were primarily influenced by AI assistance, as they represent standard, well-established patterns:

**Infrastructure & Configuration Files**:
- `Dockerfile` and `Dockerfile.consumer` - Standard multi-stage Docker builds
- `docker-compose.yml` - Service orchestration configuration
- `Makefile` - Development workflow automation
- `redis.conf` - Redis server configuration
- `.dockerignore` files - Build optimization

**Documentation**:
- Large portions of `README.md` - Standard project documentation structure
- AI was used to refine my initial draft of the readme into more structured sections.


## Troubleshooting

**Debug Commands**:
```bash
# Check container health
docker-compose ps

# View detailed logs
docker-compose logs -f [service_name]

# Interactive Redis debugging
docker-compose exec redis redis-cli

# Check consumer log files
docker-compose exec redis-consumer-1 cat /var/log/radius_updates.log
```