# 🚀 Emoji Logging Guide

Using emojis in logs helps quickly spot important messages when scanning through terminal output.  
This guide provides a consistent emoji set for log levels, performance, networking, and system events.

---

## 📊 Log Levels

| Level     | Emoji | Usage Example |
|-----------|-------|----------------|
| INFO      | ℹ️     | `ℹ️ INFO: Service started` |
| SUCCESS   | ✅     | `✅ SUCCESS: Connected to DB` |
| WARNING   | ⚠️     | `⚠️ WARN: High latency detected` |
| ERROR     | ❌     | `❌ ERROR: Failed to connect to Kafka` |
| CRITICAL  | 🔴     | `🔴 CRITICAL: Data corruption detected` |
| FATAL     | 🛑     | `🛑 FATAL: Service crashed` |
| DEBUG     | 🐛     | `🐛 DEBUG: User payload = {...}` |
| TRACE     | 🔍     | `🔍 TRACE: Entering function doWork()` |

---

## ⏱ Performance / Timing

| Emoji | Meaning | Example |
|-------|----------|---------|
| ⏱     | Timing measurement | `⏱ Took 245ms to complete query` |
| 🕒     | Timestamp log | `🕒 2025-10-03T12:34:56Z` |
| 🐇     | Fast operation | `🐇 Cache hit, returning result` |
| 🐢     | Slow operation | `🐢 Query took too long` |
| 🐌     | Bottleneck / degraded | `🐌 Slow consumer detected` |

---

## 🔗 Networking / Messaging

| Emoji | Meaning | Example |
|-------|----------|---------|
| 🌐     | Network event | `🌐 Outbound request to https://api.example.com` |
| 🔌     | Connected | `🔌 Connected to PostgreSQL` |
| 🔒     | Secured connection | `🔒 TLS handshake successful` |
| 📡     | Message sent | `📡 Produced message to topic=metrics` |
| 📥     | Message received | `📥 Consumed message from topic=orders` |
| 📤     | Message published | `📤 Event dispatched to subscribers` |
| 📶     | Connection status | `📶 Broker unreachable` |

---

## 🛠 System / Operations

| Emoji | Meaning | Example |
|-------|----------|---------|
| 🛠     | Setup / init | `🛠 Initializing worker pool` |
| 📦     | Dependency loaded | `📦 Loaded configuration from /etc/app/config.yml` |
| 🚀     | Service started / deployed | `🚀 Service running at :8080` |
| 🔄     | Restart / retry | `🔄 Retrying request (attempt 2)` |
| 🗑     | Cleanup / deleted resource | `🗑 Removing stale cache entries` |
| 🧹     | Garbage collection / cleanup | `🧹 Flushed old sessions` |
| 🧩     | Module / component log | `🧩 AuthModule: validating token` |

---

## 🧪 Testing / Development

| Emoji | Meaning | Example |
|-------|----------|---------|
| 🧪     | Test case | `🧪 Running unit tests` |
| 📝     | Log / trace details | `📝 Request headers = {...}` |
| 🎯     | Assertion / target | `🎯 Expecting 200 OK` |
| 🔧     | Debugging tools | `🔧 Profiling enabled` |

---

## 🔒 Security

| Emoji | Meaning | Example |
|-------|----------|---------|
| 🔑     | Key / authentication | `🔑 API key validated` |
| 🛡     | Security event | `🛡 Authorization passed` |
| 🚨     | Security alert | `🚨 Unauthorized access attempt` |
| 🔓     | Permission / access granted | `🔓 User logged in` |

---

## 🎉 Miscellaneous

| Emoji | Meaning | Example |
|-------|----------|---------|
| 🎉     | Success / milestone | `🎉 Deployment finished successfully` |
| 📊     | Metrics / stats | `📊 Processed 12,345 messages` |
| 🎁     | Feature flag / experiment | `🎁 Beta feature enabled` |
| 🔔     | Notification | `🔔 Job finished` |
