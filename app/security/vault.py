import os
import base64
import json
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from argon2 import PasswordHasher
from argon2.low_level import hash_secret_raw, Type
from app.core.config import logger

class VaultManager:
    _instance = None

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super(VaultManager, cls).__new__(cls, *args, **kwargs)
            cls._instance._session_key = None
            cls._instance.ph = PasswordHasher()
            # Auto unlock the vault with a static master password for direct dashboard access
            cls._instance.set_session_password("AIStorageAdvisorStaticMasterPassword123!", "c3RhdGljX3NhbHRfZm9yX3BvcnRhYmxlX21vZGU=")
        return cls._instance

    def set_session_password(self, password: str, salt_b64: str):
        """Derives a 256-bit key from the master password and salt using Argon2id."""
        try:
            salt = base64.b64decode(salt_b64.encode('utf-8'))
            # Derive 32 bytes (256 bits) key
            derived_key = hash_secret_raw(
                secret=password.encode('utf-8'),
                salt=salt,
                time_cost=3,
                memory_cost=65536,
                parallelism=4,
                hash_len=32,
                type=Type.ID
            )
            self._session_key = derived_key
            logger.info("Session key derived and loaded into memory.")
            return True
        except Exception as e:
            logger.error(f"Error deriving session key: {e}")
            return False

    def clear_session(self):
        self._session_key = None
        logger.info("Session key cleared from memory.")

    def is_unlocked(self) -> bool:
        return self._session_key is not None

    def hash_password(self, password: str) -> tuple[str, str]:
        """Hashes a password for storage and generates a salt.
        Returns (password_hash, salt_b64)."""
        salt = os.urandom(16)
        salt_b64 = base64.b64encode(salt).decode('utf-8')
        
        # Hash using Argon2
        pwd_hash = self.ph.hash(password)
        return pwd_hash, salt_b64

    def verify_password(self, pwd_hash: str, password: str) -> bool:
        """Verifies a password against its stored hash."""
        try:
            return self.ph.verify(pwd_hash, password)
        except Exception:
            return False

    def encrypt(self, plaintext: str) -> str:
        """Encrypts plaintext using AES-256-GCM.
        Returns a base64 encoded string containing nonce + ciphertext + tag."""
        if not self.is_unlocked():
            raise ValueError("Vault is locked. Session key not initialized.")
        
        nonce = os.urandom(12)
        aesgcm = AESGCM(self._session_key)
        ciphertext = aesgcm.encrypt(nonce, plaintext.encode('utf-8'), None)
        
        # Combine nonce and ciphertext
        combined = nonce + ciphertext
        return base64.b64encode(combined).decode('utf-8')

    def decrypt(self, ciphertext_b64: str) -> str:
        """Decrypts a base64 encoded string containing nonce + ciphertext + tag."""
        if not self.is_unlocked():
            raise ValueError("Vault is locked. Session key not initialized.")
        
        try:
            combined = base64.b64decode(ciphertext_b64.encode('utf-8'))
            if len(combined) < 12:
                raise ValueError("Invalid ciphertext length.")
            
            nonce = combined[:12]
            ciphertext = combined[12:]
            
            aesgcm = AESGCM(self._session_key)
            decrypted = aesgcm.decrypt(nonce, ciphertext, None)
            return decrypted.decode('utf-8')
        except Exception as e:
            logger.error(f"Decryption failed: {e}")
            raise ValueError("Decryption failed. Incorrect key or corrupted data.")

# Singleton instance
vault = VaultManager()
