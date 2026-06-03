# 🤖 AI Storage Advisor

**AI-powered desktop utility for intelligent disk storage analysis, cleanup recommendations, and remote server diagnostics.**

![Python](https://img.shields.io/badge/Python-3.12+-blue?logo=python)
![PySide6](https://img.shields.io/badge/GUI-PySide6-green?logo=qt)
![License](https://img.shields.io/badge/License-MIT-yellow)
![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?logo=windows)

---

## ✨ Features

### 🔍 Smart Disk Scanning
- **Local Scan** — scan any local folder or drive
- **SSH Remote Linux** — connect to remote Linux servers via SSH with auto-detect mode (`/var/log`, `/tmp`, `/var/tmp`, `/var/cache`)
- **Network Share (UNC)** — browse and scan Windows network shares

### 🤖 AI-Powered Recommendations
- Connect to any LLM provider: **Ollama**, **LM Studio**, **OpenAI API**, **Anthropic API**, **Gemini API**, **DeepSeek API**
- AI analyzes scan results and provides personalized cleanup recommendations in Ukrainian 🇺🇦
- Interactive inline delete actions — click to delete directly from AI advice

### 🎨 Modern UI
- **Dark & Light themes** with one-click toggle and persistent preference
- Premium glassmorphism design with smooth transitions
- Fully localized Ukrainian interface

### 🔐 Security
- AES-256-GCM encrypted credential vault
- All API keys and SSH passwords stored encrypted in local SQLite database
- Argon2id key derivation

### 📦 Portable
- Single `.exe` file — no installation required
- Just copy `AI Storage Advisor.exe` + `data/` folder to any Windows PC
- Auto-creates `data/` directory on first launch

---

## 🚀 Quick Start

### Option 1: Run from Source
```bash
# Install dependencies
pip install -r requirements.txt

# Launch
python main.py
```

### Option 2: Run Portable EXE
1. Download `AI Storage Advisor.exe` from the repository
2. Place it in any folder
3. Double-click to run — `data/` folder will be created automatically

### Option 3: Build Your Own EXE
```bash
pip install pyinstaller
python -m PyInstaller --onefile --noconsole --name="AI Storage Advisor" --icon="app_icon.ico" --clean main.py
```

---

## 📂 Project Structure

```
AI-Storage-Advisor/
├── main.py                     # Application entry point
├── app/
│   ├── core/
│   │   ├── config.py           # App configuration & portable path resolver
│   │   └── profile_manager.py  # Profile export/import
│   ├── database/
│   │   └── db_manager.py       # SQLite database manager (WAL mode)
│   ├── modules/
│   │   ├── scanner.py          # Local disk scanner with duplicate detection
│   │   ├── remote_scanner.py   # SSH remote Linux scanner
│   │   ├── cleanup_advisor.py  # Safe file deletion engine
│   │   ├── rules_engine.py     # Configurable scan rules
│   │   ├── forecast_engine.py  # Storage growth forecasting
│   │   └── sre_analyzer.py     # SRE health scoring & Docker diagnostics
│   ├── providers/
│   │   ├── base.py             # Abstract AI provider interface
│   │   ├── local_providers.py  # Ollama & LM Studio providers
│   │   └── api_providers.py    # OpenAI, Anthropic, Gemini, DeepSeek
│   ├── security/
│   │   └── vault.py            # AES-256-GCM encryption vault
│   └── ui/
│       ├── main_window.py      # Main window with theme system
│       ├── dashboard_screen.py # AI recommendations dashboard
│       ├── settings_screen.py  # Provider & SSH configuration
│       └── components/         # Reusable UI dialogs
├── app_icon.png                # Application icon (PNG)
├── app_icon.ico                # Application icon (ICO)
├── requirements.txt            # Python dependencies
├── test_backend.py             # Automated test suite (9 tests)
└── data/                       # Auto-generated portable data directory
    ├── storage_advisor.db      # SQLite database (encrypted credentials)
    └── logs/
        └── app.log             # Application log
```

---

## ⚙️ Configuration

### AI Providers
Navigate to **Settings** tab and configure your preferred AI provider:

| Provider | Type | Default Model |
|----------|------|---------------|
| Ollama | Local | llama3 |
| LM Studio | Local | meta-llama-3-8b-instruct |
| OpenAI API | Cloud | gpt-5.4-mini |
| Anthropic API | Cloud | claude-sonnet-4.6 |
| Gemini API | Cloud | gemini-3.5-flash |
| DeepSeek API | Cloud | deepseek-v4-flash |

### SSH Remote Hosts
Add remote servers in **Settings → 🖥️ Remote Hosts** or enter credentials directly on the Dashboard.

### Scan Rules
Customize cleanup thresholds in **Settings → 📋 Scan Rules**:
- Temp files older than N days
- Log files larger than N MB
- Log files older than N days
- Backups older than N days

---

## 🧪 Testing

```bash
python test_backend.py
```

Runs 9 automated tests covering:
- Security vault (AES-256-GCM encryption, Argon2id hashing)
- SQLite database schema validation
- Disk scanner with duplicate detection
- Rules engine evaluation
- Cleanup advisor (dry run & safe deletion)
- Storage forecast calculations
- Profile export/import with encryption
- SSH remote scanner parsing
- SRE health scoring algorithm

---

## 📋 Requirements

- Python 3.12+
- PySide6
- cryptography
- argon2-cffi
- psutil
- requests
- paramiko
- beautifulsoup4
- send2trash

---

## 📝 License

MIT License — see [LICENSE](LICENSE) for details.
