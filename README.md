# AI Storage Advisor 🚀

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org)
[![Wails](https://img.shields.io/badge/Wails-v2-blue?logo=wails)](https://wails.io)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)]()

**AI Storage Advisor** / **AI Радник зі Сховища**

> *AI-powered disk analysis, forecasting, and secure cleanup tool*
> *Інструмент аналізу дисків, прогнозування та безпечного очищення з використанням ШІ*

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
```powershell
go test ./... -v -count=1
```

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

## 📁 Project Structure / Структура проекту

```
AI-Storage-Advisor/
├── main.go                    # Wails app entry point
├── app.go                     # Go backend bindings
├── app_test.go                # Backend tests
├── wails.json                 # Wails configuration
├── go.mod / go.sum            # Go dependencies
├── frontend/                  # Web frontend
│   ├── index.html             # Main HTML (Ukrainian UI)
│   ├── src/
│   │   ├── main.js            # App initialization & routing
│   │   ├── api.js             # Wails Go bindings re-export
│   │   ├── state.js           # Global application state
│   │   ├── utils.js           # Utility functions
│   │   ├── app.css / style.css# Theming & styling
│   │   └── ui/
│   │       ├── scanner.js     # Scan & results rendering
│   │       ├── settings.js    # Settings forms logic
│   │       └── ssh.js         # SSH host CRUD
│   └── package.json           # Node.js dependencies
├── backend/                   # Go backend packages
│   ├── scanner/               # File system scanning
│   │   ├── scanner.go         # Local disk scanner
│   │   ├── remote.go          # SSH remote scanner
│   │   └── *_test.go          # Scanner tests
│   ├── cleanup/               # Secure file deletion
│   │   ├── cleanup.go         # Dry run & safe delete
│   │   └── cleanup_test.go    # Cleanup tests
│   ├── security/              # Encryption vault
│   │   ├── vault.go           # AES-256-GCM + Argon2id
│   │   └── vault_test.go      # Vault tests
│   ├── providers/             # AI provider integrations
│   │   └── providers.go       # OpenAI, Gemini, Ollama, etc.
│   ├── rules/                 # Scan rules engine
│   │   ├── rules.go           # Rule evaluation & processing
│   │   └── rules_test.go      # Rules tests
│   ├── sre/                   # SRE health analysis
│   │   ├── sre.go             # Docker, Windows analysis
│   │   └── sre_test.go        # SRE tests
│   ├── forecast/              # Storage forecasting
│   │   └── forecast.go        # OLS linear regression
│   ├── profile/               # Profile export/import
│   │   └── profile.go         # Encrypted backup
│   ├── config/                # Application configuration
│   │   ├── config.go          # Cross-platform config init
│   │   ├── config_windows.go  # Windows-specific
│   │   ├── config_other.go    # Unix-specific
│   │   └── config_test.go     # Config tests
│   └── db/                    # SQLite database
│       └── db.go              # Schema & initialization
├── windows/                   # Windows resources
├── darwin/                    # macOS resources
└── build/                     # Build output
```

---

## 📄 License / Ліцензія

This project is licensed under the MIT License — see the LICENSE file for details.
Цей проект ліцензовано за ліцензією MIT — див. файл LICENSE для деталей.

---

## 👨‍💻 Author / Автор

**ajjs1ajjs** — [ajjs1ajjs@users.noreply.github.com](mailto:ajjs1ajjs@users.noreply.github.com)
