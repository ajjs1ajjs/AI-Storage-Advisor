import os
import sys
import logging
from pathlib import Path

# Application Constants
APP_NAME = "AI Storage Advisor"
VERSION = "0.1"

# Determine base app directory (where data is stored)
if getattr(sys, 'frozen', False):
    # Running as a compiled executable
    APP_ROOT = Path(sys.executable).parent.absolute()
else:
    # Running in standard Python environment
    APP_ROOT = Path(__file__).parent.parent.parent.absolute()

# Force portable mode by keeping data directory next to the application
APP_DATA_DIR = APP_ROOT / "data"

# Ensure directories exist
APP_DATA_DIR.mkdir(parents=True, exist_ok=True)
DB_PATH = APP_DATA_DIR / "storage_advisor.db"
VAULT_PATH = APP_DATA_DIR / "security_vault.enc"
LOG_DIR = APP_DATA_DIR / "logs"
LOG_DIR.mkdir(parents=True, exist_ok=True)
LOG_FILE = LOG_DIR / "app.log"

# Configure logging
handlers = [logging.FileHandler(LOG_FILE, encoding="utf-8")]
if sys.stdout is not None:
    handlers.append(logging.StreamHandler(sys.stdout))

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    handlers=handlers
)

logger = logging.getLogger("AIStorageAdvisor")
logger.info(f"Initialized application config. Data dir: {APP_DATA_DIR}")
