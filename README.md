# AI Storage Advisor 🚀

[![CI](https://github.com/ajjs1ajjs/AI-Storage-Advisor/actions/workflows/ci.yml/badge.svg)](https://github.com/ajjs1ajjs/AI-Storage-Advisor/actions/workflows/ci.yml)
[![CodeQL](https://github.com/ajjs1ajjs/AI-Storage-Advisor/actions/workflows/codeql.yml/badge.svg)](https://github.com/ajjs1ajjs/AI-Storage-Advisor/actions/workflows/codeql.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ajjs1ajjs/AI-Storage-Advisor)](https://goreportcard.com/report/github.com/ajjs1ajjs/AI-Storage-Advisor)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8)](https://golang.org)
[![Tests](https://img.shields.io/badge/tests-200%2B-brightgreen)](https://github.com/ajjs1ajjs/AI-Storage-Advisor/actions)

**AI Storage Advisor** / **AI Радник зі Сховища**

> *AI-powered disk analysis, forecasting, and secure cleanup tool*
> *Інструмент аналізу дисків, прогнозування та безпечного очищення з використанням ШІ*

---

### 🏆 Features

- **Wails v2** — Go backend + Vanilla JS frontend, native desktop app
- **Virtual Scrolling** — 100k+ files without lag (VirtualScroller)
- **End-to-end Encryption** — AES-256-GCM + Argon2id for credentials & profiles
- **Forecasting** — Linear regression for storage growth prediction
- **SSH Remote Scanning** — Connect to remote Linux hosts for analysis
- **AI Integration** — OpenAI, Anthropic, Gemini, DeepSeek, Ollama, LM Studio
- **Windows Recycle Bin** — Safe deletion via `IFileOperation`
- **SRE Health Score** — Storage reliability metrics (duplicates, logs, temp, SRE)
- **CI/CD** — GitHub Actions (Go + Node matrix), CodeQL, golangci-lint, Lefthook

---

## 📋 Features / Можливості

### English
- **Deep Disk Scanning** (Local / UNC / SSH)
  - Multi-threaded local disk scanning for maximum SSD performance
  - Remote Linux server scanning via SSH with key/password auth
- **SRE Health Score** — Docker container analysis, package cache detection
- **Smart Duplicate Checker** — xxhash-based fast duplicate detection
- **Storage Forecast (OLS Regression)** — predicts disk exhaustion
- **AI Integration** — OpenAI, Gemini, Anthropic, DeepSeek, Ollama, LM Studio
  - Multi-language AI recommendations (Ukrainian, English, Spanish)
  - Interactive AI chat for follow-up questions
  - Automated cleanup action buttons from AI recommendations
- **Secure Deletion** — Windows Recycle Bin integration (SHFileOperationW)
- **Encrypted Vault** — AES-256-GCM with Argon2id key derivation
- **Profile Import/Export** — encrypted backup of settings and credentials

### Українська
- **Глибоке сканування дисків** (Локальний / UNC / SSH)
  - Багатопотокове сканування локальних накопичувачів
  - Віддалене сканування Linux серверів через SSH
- **SRE Health Score** — аналіз Docker контейнерів, кешів пакетів
- **Розумний пошук дублікатів** — швидке виявлення через xxhash
- **Прогнозування сховища (OLS)** — передбачення вичерпання диска
- **Інтеграція з ШІ** — OpenAI, Gemini, Anthropic, DeepSeek, Ollama, LM Studio
  - Багатомовні рекомендації (Українська, English, Español)
  - Інтерактивний чат з ШІ
  - Кнопки автоматичних дій з рекомендацій
- **Безпечне видалення** — інтеграція з Кошиком Windows
- **Зашифроване сховище** — AES-256-GCM + Argon2id
- **Експорт/Імпорт профілів** — зашифроване резервне копіювання

---

## 🏗 Architecture / Архітектура

```
┌─────────────────────────────────────────────────────┐
│                   Frontend (Wails)                    │
│  HTML5 + Vanilla JS (ES6 modules) + CSS3 + Vite      │
│  src/main.js → state.js, api.js, ui/{scanner,ssh,    │
│              settings}.js                             │
├─────────────────────────────────────────────────────┤
│                  Backend (Go 1.25)                     │
│  app.go → HTTP bindings for Wails runtime             │
│  ├── backend/scanner/   → Local & SSH file scanning   │
│  ├── backend/cleanup/   → Secure file deletion        │
│  ├── backend/forecast/  → OLS storage forecast        │
│  ├── backend/sre/       → Docker, Windows SRE data    │
│  ├── backend/rules/     → Configurable scan rules     │
│  ├── backend/providers/ → AI provider integrations    │
│  ├── backend/security/  → AES-256-GCM + Argon2 vault  │
│  ├── backend/profile/   → Profile export/import       │
│  ├── backend/config/    → Cross-platform config        │
│  └── backend/db/        → SQLite (modernc.org/pure Go)│
└─────────────────────────────────────────────────────┘
```

### Tech Stack / Стек технологій
| Component | Technology |
|-----------|-----------|
| Desktop Framework | [Wails v2](https://wails.io) (Go + Web UI) |
| Frontend | HTML5, Vanilla JS, CSS3, [Vite](https://vitejs.dev) |
| Backend | [Go](https://golang.org) 1.25+ |
| Database | [SQLite](https://sqlite.org) via [modernc.org/sqlite](https://modernc.org/sqlite) (pure Go, no CGO) |
| Crypto | AES-256-GCM + [Argon2id](https://pkg.go.dev/golang.org/x/crypto/argon2) |
| SSH | [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) |
| Duplicates | [xxhash](https://github.com/cespare/xxhash) |
| System | [gopsutil](https://github.com/shirou/gopsutil) |

---

## 🚀 Build & Run / Збірка та запуск

### Requirements / Вимоги
- [Go](https://golang.org) 1.25+
- [Node.js](https://nodejs.org) 16+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation):
  ```powershell
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Build / Збірка
```powershell
wails build
```

The compiled binary will be at / Зібраний файл буде в:
`build/bin/AI Storage Advisor.exe`

### Development / Розробка
```powershell
wails dev
```

### Run Tests / Запуск тестів
\`\`\`powershell
go test ./... -v -count=1
\`\`\`

---

### 🧪 Testing

\`\`\`
# Run all Go tests
go test ./... -count=1

# Run benchmarks
go test -bench=. -benchmem ./backend/scanner/ ./backend/security/ ./backend/forecast/

# Run frontend tests
cd frontend && npm test

# Run fuzz tests
go test -fuzz=FuzzEncryptDecrypt ./backend/security/
go test -fuzz=FuzzDecryptInvalid ./backend/security/
\`\`\`

---

## 🧪 Testing / Тестування

The project includes comprehensive tests / Проект включає комплексні тести:

| Package | Coverage |
|---------|----------|
| `backend/scanner` | Local scan, cancellation, duplicates, rules, progress |
| `backend/security` | Vault init/unlock, Encrypt/Decrypt, archive crypto |
| `backend/sre` | Docker size parsing, health score calculation |
| `backend/rules` | Default rules, file evaluation, rule processing |
| `backend/cleanup` | Format size, dry run, safe delete |
| `backend/config` | Configuration initialization and fallback |
| `app` | Package cache command whitelist validation |

---

## 🔒 Security / Безпека

| Feature | Implementation |
|---------|---------------|
| **Vault** | AES-256-GCM session key derived via Argon2id |
| **SSH Keys** | Encrypted with vault session key before storage |
| **AI API Keys** | Encrypted with vault session key |
| **Profile Export** | AES-256-GCM with password-derived key |
| **SSH Host Keys** | TOFU (Trust On First Use) stored in DB |
| **Command Injection** | Strict whitelist for package cache commands |

### English
All sensitive data (SSH passwords, API keys, key passphrases) are encrypted before storage in SQLite using AES-256-GCM. The encryption key is derived from the master password using Argon2id and never stored on disk.

### Українська
Всі чутливі дані (SSH паролі, API ключі, passphrase ключів) шифруються перед збереженням у SQLite за допомогою AES-256-GCM. Ключ шифрування створюється з master-пароля через Argon2id.

---

## 🌐 Language Support / Підтримка мов

- **Українська** — Default UI language / Мова інтерфейсу за замовчуванням
- **English** — Available for AI recommendations / Доступна для рекомендацій ШІ
- **Español** — Available for AI recommendations / Доступна для рекомендацій ШІ

AI language can be changed in Settings → AI Providers → Language.
Мову ШІ можна змінити в Налаштуваннях → AI Провайдери → Мова.

---

### 📁 Project Structure

\`\`\`
.
├── app.go                          # Wails app bindings (~1105 lines)
├── app_test.go / app_ssh_test.go   # Tests for app functions
├── backend/
│   ├── cleanup/       # Safe file deletion (Windows recycle bin)
│   ├── config/        # App configuration & paths
│   ├── db/            # SQLite database init & schema
│   ├── forecast/      # Storage growth forecasting
│   ├── logger/        # Structured logging (DEBUG/INFO/WARN/ERROR/FATAL)
│   ├── profile/       # Profile export/import with encryption
│   ├── providers/     # AI providers (OpenAI, Anthropic, Gemini, etc.)
│   ├── rules/         # Cleanup rules engine
│   ├── scanner/       # Disk scanner (local + SSH + MD5 dedup)
│   ├── security/      # AES-256-GCM vault + Argon2id key derivation
│   ├── sre/           # Health score calculation
│   └── utils/         # Shared utilities (FormatSize)
├── frontend/
│   ├── src/
│   │   ├── ui/        # UI components (scanner, settings, ssh, virtual-scroll)
│   │   ├── state.js   # Global reactive state
│   │   ├── utils.js   # Frontend helpers
│   │   └── __tests__/ # Vitest tests (50+ tests)
│   ├── package.json
│   └── vite.config.js
├── .github/workflows/  # CI/CD (ci.yml, codeql.yml)
├── .golangci.yml       # Go linter config
├── lefthook.yml        # Pre-commit hooks
├── Dockerfile          # Dev/CI environment
└── README.md           # Documentation (UA + EN)
\`\`\`

---

## 📄 License / Ліцензія

This project is licensed under the MIT License — see the LICENSE file for details.
Цей проект ліцензовано за ліцензією MIT — див. файл LICENSE для деталей.

---

## 👨‍💻 Author / Автор

**ajjs1ajjs** — [ajjs1ajjs@users.noreply.github.com](mailto:ajjs1ajjs@users.noreply.github.com)
