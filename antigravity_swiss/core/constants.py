"""
Antigravity Swiss Knife Core Constants and Defaults.
====================================================
Contains Material 3 color tokens, default file system locations,
socket and wire protocol parameters, and Google API schemas.
"""

from pathlib import Path

# Application Metadata
APP_NAME = "antigravity-swiss"
APP_TITLE = "Antigravity Swiss Knife"
APP_VERSION = "0.1.0"
APP_AUTHOR = "ChillingWombat"

# Linux Secret Service / Keyring Constants
KEYRING_SERVICE_NAME = "gemini"
KEYRING_USERNAME = "antigravity"
KEYRING_SCHEMA_NAME = "org.freedesktop.Secret.Generic"
KEYRING_LABEL = "Password for 'antigravity' on 'gemini'"

# Default Host Antigravity Paths
DEFAULT_ANTIGRAVITY_BIN = Path("/opt/Antigravity/antigravity")
DEFAULT_ANTIGRAVITY_CONFIG_DIR = Path.home() / ".config" / "Antigravity"
DEFAULT_ANTIGRAVITY_DATA_DIR = Path.home() / ".gemini" / "antigravity"
DEFAULT_SWISS_CONFIG_DIR = Path.home() / ".config" / APP_NAME
DEFAULT_SWISS_DATA_DIR = Path.home() / ".local" / "share" / APP_NAME

# Antigravity File Names
SINGLETON_LOCK_NAME = "SingletonLock"
SINGLETON_SOCKET_NAME = "SingletonSocket"
SINGLETON_COOKIE_NAME = "SingletonCookie"
APP_STORAGE_JSON_NAME = "app_storage.json"
CONVERSATION_SUMMARIES_DB_NAME = "conversation_summaries.db"
STATE_VSCDB_NAME = "state.vscdb"
STATE_VSCDB_WAL_NAME = "state.vscdb-wal"

# Fingerprint File Names
FINGERPRINT_MACHINE_ID = "machineid"
FINGERPRINT_UPDATER_ID = ".updaterId"
FINGERPRINT_INSTALLATION_ID = "installation_id"
FINGERPRINT_PBTXT = "antigravity_state.pbtxt"

# Environment Variable Overrides
ENV_CONFIG_DIR = "ANTIGRAVITY_SWISS_CONFIG_DIR"
ENV_SOCKET_PATH = "ANTIGRAVITY_SWISS_SOCKET_PATH"
ENV_ACCOUNTS_FILE = "ANTIGRAVITY_SWISS_ACCOUNTS_FILE"
ENV_SETTINGS_FILE = "ANTIGRAVITY_SWISS_SETTINGS_FILE"
ENV_POLL_INTERVAL = "ANTIGRAVITY_SWISS_POLL_INTERVAL"
ENV_SWITCH_THRESHOLD = "ANTIGRAVITY_SWISS_SWITCH_THRESHOLD"
ENV_AUTO_SWITCH_ENABLED = "ANTIGRAVITY_SWISS_AUTO_SWITCH"
ENV_WARMUP_ENABLED = "ANTIGRAVITY_SWISS_WARMUP_ENABLED"
ENV_ANTIGRAVITY_BIN = "ANTIGRAVITY_BIN_PATH"
ENV_ANTIGRAVITY_CONFIG = "ANTIGRAVITY_CONFIG_DIR"
ENV_ANTIGRAVITY_DATA = "ANTIGRAVITY_DATA_DIR"

# IPC Socket & Wire Specs
SOCKET_DIR_NAME = "antigravity-swiss"
SOCKET_FILE_NAME = "daemon.sock"
SOCKET_DIR_MODE = 0o700
SOCKET_FILE_MODE = 0o600
MAX_SOCKET_PATH_LEN = 104  # Safe margin under Linux 108 limit
MAX_FRAME_SIZE = 10 * 1024 * 1024  # 10 MB limit for JSON-RPC messages
JSONRPC_VERSION = "2.0"
WIRE_DELIMITER = b"\n"

# Default Tuning & Operation Parameters
DEFAULT_POLLING_INTERVAL_SECONDS = 60.0
DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION = 0.05  # 5% quota
DEFAULT_WARMUP_LEAD_TIME_SECONDS = 2.0
DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS = 10.0
DEFAULT_CLIENT_TIMEOUT_SECONDS = 15.0
DEFAULT_WARMUP_MODEL_ID = "gemini-3.8-flash-high"
DEFAULT_MAX_TOKENS_WARMUP = 1

# Google Upstream Endpoints & OAuth
GOOGLE_OAUTH_TOKEN_URL = "https://oauth2.googleapis.com/token"
GOOGLE_QUOTA_SUMMARY_URL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
GOOGLE_AVAILABLE_MODELS_URL = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
GOOGLE_GENERATE_CONTENT_URL = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
GOOGLE_DEFAULT_CLIENT_ID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
GOOGLE_DEFAULT_CLIENT_SECRET = "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"
GOOGLE_USER_AGENT = "antigravity/2.18.1"

# Material Design 3 Dark Theme Tokens (Google Gemini Palette)
MD3_SURFACE = "#131314"
MD3_SURFACE_CONTAINER = "#1e1f20"
MD3_SURFACE_CONTAINER_HIGH = "#282a2c"
MD3_ACCENT_PRIMARY = "#8ab4f8"
MD3_TEXT_PRIMARY = "#e3e3e3"
MD3_TEXT_SECONDARY = "#9aa0a6"
MD3_OUTLINE = "#3c4043"
MD3_COLOR_HEALTHY = "#81c995"
MD3_COLOR_WARNING = "#fdd663"
MD3_COLOR_EXHAUSTED = "#f28b82"
MD3_RADIUS_CARD = 16
MD3_RADIUS_PILL = 18

# Material Design 3 Light Theme Tokens (Google Gemini Light Palette)
MD3_LIGHT_SURFACE = "#f0f4f9"
MD3_LIGHT_SURFACE_CONTAINER = "#ffffff"
MD3_LIGHT_SURFACE_CONTAINER_HIGH = "#e9eef6"
MD3_LIGHT_ACCENT_PRIMARY = "#0b57d0"
MD3_LIGHT_ACCENT_CONTAINER = "#c2e7ff"
MD3_LIGHT_ACCENT_ON_CONTAINER = "#001d35"
MD3_LIGHT_TEXT_PRIMARY = "#1f1f1f"
MD3_LIGHT_TEXT_SECONDARY = "#444746"
MD3_LIGHT_OUTLINE = "#d3dbe5"
MD3_LIGHT_COLOR_HEALTHY = "#137333"
MD3_LIGHT_COLOR_WARNING = "#b06000"
MD3_LIGHT_COLOR_EXHAUSTED = "#b3261e"

