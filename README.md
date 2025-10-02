# 🚀 Simple Securities Microservices

> `Simple Securities` is an **imaginary and practical trading microservices system**, built with **Golang** and advanced architecture styles such as **Hexagonal Architecture**, **Domain Driven Design (DDD)**, **CQRS**, **Event Sourcing**, and **Event Driven Architecture**. It uses **Kafka** for messaging, **gRPC + gRPC Gateway** for service communication, and modern tooling for observability, configuration, and testing.

💡 This project is designed as a **reference implementation** and technical playground, not a production-ready exchange.

🚀 The application is **in progress** and evolves with new features and improvements.

---

## ✨ Features

- ✅ **Hexagonal Architecture** (ports & adapters)
- ✅ **Domain Driven Design (DDD)**
- ✅ **CQRS Pattern** (Command Query Responsibility Segregation)
- ✅ **Event Sourcing** (durable event store)
- ✅ **Event Driven Architecture** with **Kafka**
- ✅ **Change Data Capture** using **Debezium** + Kafka
- ✅ **gRPC** for internal service communication
- ✅ **gRPC Gateway** for HTTP/REST routing
- ✅ **Postgres & EventStoreDB** for transactional writes (ACID)
- ✅ **MongoDB & ElasticSearch** for read models (NoSQL, search)
- ✅ **Unit Testing** with mocks using **Mockery**
- ✅ **Integration & End-to-End Testing** with **testcontainers-go**
- ✅ **Zap** structured logging
- ✅ **Viper** configuration management
- ✅ **Docker & Docker Compose** for local deployment
- 🚧 **Kubernetes & Helm** (future work)

---

## 🏗️ Services

- 🔑 **core-service** → manages users, wallets, and permissions
- 💰 **crypto-service** → handles crypto orders, real-time prices via websockets, margin, futures, and crypto portfolios
- 📈 **stock-service** → manages stock orders, real-time stock prices, buy/sell operations, and stock portfolios
- 🔔 **notification-service** → push notifications, fetch by user or id
- 🌐 **gateway-service** → REST gateway using **grpc-gateway** for routing

---

## 🛠️ Tech Stack

- **Language**: Go 1.25.0
- **Service Communication**: gRPC + gRPC Gateway
- **Messaging**: Kafka
- **CDC**: Debezium → Kafka
- **Databases**:
  - Postgres (write DB)
  - EventStoreDB (event sourcing)
  - MongoDB (read DB)
  - ElasticSearch (search)
- **Config**: Viper
- **Logging**: Zap
- **Testing**: testify, mockery, testcontainers-go
- **Deployment**: Docker & Docker Compose

---

## 📚 Technologies & Libraries

Here’s a list of key libraries and tools used across services:

### 🔹 Core Libraries
- [**gRPC**](https://grpc.io/) — RPC communication between services
- [**grpc-gateway**](https://github.com/grpc-ecosystem/grpc-gateway) — REST ↔ gRPC mapping
- [**protobuf**](https://developers.google.com/protocol-buffers) — schema definitions
- [**Viper**](https://github.com/spf13/viper) — configuration management
- [**Zap**](https://github.com/uber-go/zap) — structured logging
- [**Mockery**](https://github.com/vektra/mockery) — generating mocks for tests
- [**testify**](https://github.com/stretchr/testify) — unit testing assertions
- [**testcontainers-go**](https://github.com/testcontainers/testcontainers-go) — integration testing with real containers

### 🔹 Databases & Messaging
- [**Postgres**](https://www.postgresql.org/) — relational DB for writes
- [**EventStoreDB**](https://www.eventstore.com/) — event sourcing store
- [**MongoDB**](https://www.mongodb.com/) — read DB (NoSQL)
- [**ElasticSearch**](https://www.elastic.co/) — search engine
- [**Kafka**](https://kafka.apache.org/) — event streaming
- [**Debezium**](https://debezium.io/) — change data capture

### 🔹 Dev & Ops
- [**Docker**](https://www.docker.com/) — containerization
- [**Docker Compose**](https://docs.docker.com/compose/) — local orchestration
- [**Air**](https://github.com/cosmtrek/air) — live reload for Go
- [**Make**](https://www.gnu.org/software/make/) — build automation

---

## 📐 Architecture

![Hexagonal Architecture](https://github.com/Sairyss/domain-driven-hexagon/raw/master/assets/images/DomainDrivenHexagon.png)

### 🔹 Hexagonal Architecture
- **Domain**: aggregates, entities, domain events
- **Application**: commands, queries, handlers
- **Adapters**: persistence (Postgres, EventStoreDB), messaging (Kafka), transport (gRPC, HTTP)

### 🔹 CQRS & Event Sourcing
- Commands mutate state, events are persisted in EventStoreDB
- Events are published to Kafka for other services
- Read models built asynchronously in MongoDB / ElasticSearch

### 🔹 Change Data Capture (CDC)
- Debezium streams Postgres changes into Kafka
- Enables reliable read-model updates & audit logs

---

## 🧪 Testing

### Unit Tests
```bash
go test ./... -v
```
Mocks generated with **mockery**.

### Integration & E2E Tests
```bash
make test-integration
```
Uses **testcontainers-go** to spin up real dependencies.

---

## 📊 Observability
- **Logging**: Zap (structured, leveled)
- **Tracing/Metrics**: OpenTelemetry (Jaeger, Prometheus, Grafana)

---

## 🐳 Deployment
- Dockerfiles per service (multi-stage builds)
- `docker-compose` for local development
- Future: Kubernetes + Helm

---

## 🤝 Contributing

1. Follow [Conventional Commits](https://www.conventionalcommits.org/)
2. Run `make format && make lint` before commits
3. Add tests for new features
4. Open issues for large changes

---

## 📜 License

MIT License — see [LICENSE](./LICENSE)