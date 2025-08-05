# 📈 Stock Spotlight

> A blazing-fast, local-first stock ticker app built in Go — featuring HTMX, Bootstrap, rotating logs, health checks, and real-time polling from the Polygon.io API.

---

## 🧩 Phase 1 Architecture Overview

### 💡 Purpose

This app showcases a production-grade architecture for a low-latency, real-time financial dashboard. It demonstrates:
- Go-based backend services
- HTMX-based frontend (no JS frameworks)
- In-memory caching
- Modular Go packages
- Error and retry handling
- `/healthz` endpoint
- Rotating logs
- Alert script for API rate limits

---

![Phase 1 Architecture Diagram](docs/diagram-export.png)


## 🛠️ Core Features

| Feature                      | Description                                                  |
|-----------------------------|--------------------------------------------------------------|
| ✅ Real-time stock data      | HTMX polls Go backend every 15s                              |
| ✅ Go-based caching layer    | Prevents repeated API calls to reduce usage & boost speed    |
| ✅ Snapshot polling engine   | Periodically polls Polygon API in the background             |
| ✅ Retry on API failure      | Automatically retries failed API requests                    |
| ✅ Alert script              | Sends alert/logs if API rate limit is hit                    |
| ✅ `/healthz` endpoint       | Simple JSON response to monitor service health               |
| ✅ Log rotation              | Keeps logs under size limit for production hygiene           |

---

## 🌐 API Integration

- **Source**: [Polygon.io](https://polygon.io/)
- **Usage**: Snapshot endpoint for real-time stock prices
- **Security**: API key stored in config, not hardcoded

---

## 🖥️ UI Technology

| Tech       | Use Case                            |
|------------|-------------------------------------|
| **HTMX**   | Handles polling & event handling    |
| **Bootstrap 5** | Clean, responsive styling     |
| **HTML5**  | Static templates                    |

---

## 🚦 Status Check

- 🔌 Health Endpoint: [`/healthz`](http://localhost:8080/healthz)
- 📄 Rotating Log File: `./logs/app.log`
- 🚨 Alerts: `scripts/alert.sh` triggered on API rate limit events

---

## 🧱 Directory Structure

```yaml
stockspotlight/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── api/                 # External API client (Polygon)
│   ├── cache/               # In-memory cache logic
│   ├── logger/              # Rotating log setup
│   ├── config/              # Config structs & loader
│   └── health/              # /healthz logic
├── static/
│   ├── index.html           # HTMX + Bootstrap UI
│   └── styles.css           # Custom styles (optional)
├── logs/
│   └── app.log              # Rotating logs
├── scripts/
│   └── alert.sh             # Alerts for rate limit
├── config/
│   └── app.yaml             # Polygon API key, polling intervals
├── .gitignore
├── .gitattributes
├── README.md
└── go.mod / go.sum


### Configuration

Before running the app, copy `config/app.yaml_example` to `config/app.yaml` and add your [Polygon.io](https://polygon.io) API key:

```bash
cp config/app.yaml_example config/app.yaml
