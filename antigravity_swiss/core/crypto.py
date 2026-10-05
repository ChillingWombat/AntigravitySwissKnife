"""
Cryptographic Utilities & Credential Encryption for Antigravity Swiss Knife.
=============================================================================
Provides:
1. AES-256-GCM authenticated encryption/decryption for credentials (tokens, passwords, TOTP secrets).
2. App Access Password hashing and verification (PBKDF2-HMAC-SHA256).
3. Machine-anchored master vault key derivation and caching.
"""

from __future__ import annotations

import base64
import ctypes
import ctypes.util
import hashlib
import hmac
import json
import logging
import os
import re
from pathlib import Path
from typing import Optional, Tuple

logger = logging.getLogger("antigravity_swiss.crypto")

# Wire prefix for all encrypted fields
ENCRYPTED_PREFIX = "enc:v1:"

# PBKDF2 parameters for App Password and Key Derivation
PBKDF2_ROUNDS = 100_000
KEY_LEN = 32  # 256 bits

# OpenSSL GCM Ctrl Constants
EVP_CTRL_GCM_SET_IVLEN = 0x9
EVP_CTRL_GCM_GET_TAG = 0x10
EVP_CTRL_GCM_SET_TAG = 0x11

_crypto_lib: Optional[ctypes.CDLL] = None
_lib_initialized = False


def _get_crypto_lib() -> Optional[ctypes.CDLL]:
    global _crypto_lib, _lib_initialized
    if _lib_initialized:
        return _crypto_lib

    _lib_initialized = True
    try:
        lib_name = ctypes.util.find_library("crypto")
        if lib_name:
            lib = ctypes.CDLL(lib_name)
            # Setup 64-bit argument & return types
            lib.EVP_CIPHER_CTX_new.restype = ctypes.c_void_p
            lib.EVP_CIPHER_CTX_free.argtypes = [ctypes.c_void_p]
            lib.EVP_aes_256_gcm.restype = ctypes.c_void_p
            _crypto_lib = lib
    except Exception as exc:
        logger.warning("Could not load native OpenSSL libcrypto: %s", exc)
        _crypto_lib = None

    return _crypto_lib


def _encrypt_aes_gcm_native(lib: ctypes.CDLL, key: bytes, plaintext: bytes) -> bytes:
    """Encrypts plaintext using OpenSSL AES-256-GCM."""
    iv = os.urandom(12)
    ctx = lib.EVP_CIPHER_CTX_new()
    cipher = lib.EVP_aes_256_gcm()

    try:
        lib.EVP_EncryptInit_ex(ctypes.c_void_p(ctx), ctypes.c_void_p(cipher), None, None, None)
        lib.EVP_CIPHER_CTX_ctrl(ctypes.c_void_p(ctx), EVP_CTRL_GCM_SET_IVLEN, len(iv), None)
        lib.EVP_EncryptInit_ex(ctypes.c_void_p(ctx), None, None, key, iv)

        out = (ctypes.c_ubyte * (len(plaintext) + 16))()
        outlen = ctypes.c_int()
        lib.EVP_EncryptUpdate(ctypes.c_void_p(ctx), out, ctypes.byref(outlen), plaintext, len(plaintext))

        final_out = (ctypes.c_ubyte * 16)()
        final_outlen = ctypes.c_int()
        lib.EVP_EncryptFinal_ex(ctypes.c_void_p(ctx), final_out, ctypes.byref(final_outlen))

        tag = (ctypes.c_ubyte * 16)()
        lib.EVP_CIPHER_CTX_ctrl(ctypes.c_void_p(ctx), EVP_CTRL_GCM_GET_TAG, 16, tag)

        ciphertext = bytes(out[: outlen.value]) + bytes(final_out[: final_outlen.value])
        # Return format: iv (12) + ciphertext + tag (16)
        return iv + ciphertext + bytes(tag)
    finally:
        lib.EVP_CIPHER_CTX_free(ctypes.c_void_p(ctx))


def _decrypt_aes_gcm_native(lib: ctypes.CDLL, key: bytes, payload: bytes) -> bytes:
    """Decrypts payload (iv (12) + ciphertext + tag (16)) using OpenSSL AES-256-GCM."""
    if len(payload) < 28:
        raise ValueError("Invalid ciphertext length for AES-GCM")

    iv = payload[:12]
    tag = payload[-16:]
    ciphertext = payload[12:-16]

    ctx = lib.EVP_CIPHER_CTX_new()
    cipher = lib.EVP_aes_256_gcm()

    try:
        lib.EVP_DecryptInit_ex(ctypes.c_void_p(ctx), ctypes.c_void_p(cipher), None, None, None)
        lib.EVP_CIPHER_CTX_ctrl(ctypes.c_void_p(ctx), EVP_CTRL_GCM_SET_IVLEN, len(iv), None)
        lib.EVP_DecryptInit_ex(ctypes.c_void_p(ctx), None, None, key, iv)

        out = (ctypes.c_ubyte * (len(ciphertext) + 16))()
        outlen = ctypes.c_int()
        lib.EVP_DecryptUpdate(ctypes.c_void_p(ctx), out, ctypes.byref(outlen), ciphertext, len(ciphertext))

        lib.EVP_CIPHER_CTX_ctrl(ctypes.c_void_p(ctx), EVP_CTRL_GCM_SET_TAG, len(tag), tag)

        final_out = (ctypes.c_ubyte * 16)()
        final_outlen = ctypes.c_int()
        ret = lib.EVP_DecryptFinal_ex(ctypes.c_void_p(ctx), final_out, ctypes.byref(final_outlen))
        if ret <= 0:
            raise ValueError("Authentication tag mismatch or corrupted ciphertext")

        return bytes(out[: outlen.value]) + bytes(final_out[: final_outlen.value])
    finally:
        lib.EVP_CIPHER_CTX_free(ctypes.c_void_p(ctx))


def _pure_python_ctr_hmac_encrypt(key: bytes, plaintext: bytes) -> bytes:
    """Fallback CTR stream cipher + HMAC-SHA256 authenticated envelope."""
    iv = os.urandom(12)
    # Derive encryption and MAC subkeys
    enc_key = hashlib.sha256(key + b":enc").digest()
    mac_key = hashlib.sha256(key + b":mac").digest()

    blocks = []
    counter = 0
    for i in range(0, len(plaintext), 32):
        counter += 1
        keystream = hmac.new(enc_key, iv + counter.to_bytes(4, "big"), hashlib.sha256).digest()
        chunk = plaintext[i : i + 32]
        blocks.append(bytes(a ^ b for a, b in zip(chunk, keystream)))

    ciphertext = b"".join(blocks)
    tag = hmac.new(mac_key, iv + ciphertext, hashlib.sha256).digest()[:16]
    return iv + ciphertext + tag


def _pure_python_ctr_hmac_decrypt(key: bytes, payload: bytes) -> bytes:
    """Fallback CTR stream cipher + HMAC-SHA256 decryption."""
    if len(payload) < 28:
        raise ValueError("Invalid payload length")

    iv = payload[:12]
    tag = payload[-16:]
    ciphertext = payload[12:-16]

    enc_key = hashlib.sha256(key + b":enc").digest()
    mac_key = hashlib.sha256(key + b":mac").digest()

    expected_tag = hmac.new(mac_key, iv + ciphertext, hashlib.sha256).digest()[:16]
    if not hmac.compare_digest(tag, expected_tag):
        raise ValueError("HMAC authentication failed")

    blocks = []
    counter = 0
    for i in range(0, len(ciphertext), 32):
        counter += 1
        keystream = hmac.new(enc_key, iv + counter.to_bytes(4, "big"), hashlib.sha256).digest()
        chunk = ciphertext[i : i + 32]
        blocks.append(bytes(a ^ b for a, b in zip(chunk, keystream)))

    return b"".join(blocks)


def get_default_vault_key(config_dir: Optional[Path] = None) -> bytes:
    """
    Retrieves or generates the local machine vault encryption key.
    Stored with strict 0600 permissions in ~/.config/antigravity-swiss/.vault_key.
    """
    if config_dir is None:
        from antigravity_swiss.core.constants import DEFAULT_SWISS_CONFIG_DIR
        config_dir = Path(DEFAULT_SWISS_CONFIG_DIR)

    config_dir.mkdir(parents=True, exist_ok=True)
    key_file = config_dir / ".vault_key"

    if key_file.exists():
        try:
            raw = key_file.read_bytes().strip()
            if len(raw) == KEY_LEN:
                return raw
            # Try decoding base64 if stored as text
            decoded = base64.b64decode(raw)
            if len(decoded) == KEY_LEN:
                return decoded
        except Exception:
            pass

    # Generate new random 256-bit vault key
    new_key = os.urandom(KEY_LEN)
    try:
        # Atomic write with 0600 permissions
        tmp_file = key_file.with_suffix(".tmp")
        tmp_file.write_bytes(new_key)
        tmp_file.chmod(0o600)
        tmp_file.replace(key_file)
    except Exception as exc:
        logger.warning("Could not persist vault key to %s: %s", key_file, exc)

    return new_key


def encrypt_credential(plaintext: str, key: Optional[bytes] = None) -> str:
    """
    Encrypts a sensitive string (token, password, totp secret).
    Returns ciphertext prefixed with 'enc:v1:<base64>'.
    Idempotent: if already prefixed with 'enc:v1:', returns as is.
    """
    if not plaintext:
        return ""
    if plaintext.startswith(ENCRYPTED_PREFIX):
        return plaintext

    if key is None:
        key = get_default_vault_key()

    data = plaintext.encode("utf-8")
    lib = _get_crypto_lib()
    if lib:
        try:
            packed = _encrypt_aes_gcm_native(lib, key, data)
        except Exception:
            packed = _pure_python_ctr_hmac_encrypt(key, data)
    else:
        packed = _pure_python_ctr_hmac_encrypt(key, data)

    return ENCRYPTED_PREFIX + base64.b64encode(packed).decode("ascii")


def decrypt_credential(ciphertext: str, key: Optional[bytes] = None) -> str:
    """
    Decrypts a ciphertext string prefixed with 'enc:v1:<base64>'.
    If the string is plaintext (does not start with 'enc:v1:'), returns it untouched.
    """
    if not ciphertext:
        return ""
    if not ciphertext.startswith(ENCRYPTED_PREFIX):
        return ciphertext

    if key is None:
        key = get_default_vault_key()

    raw_b64 = ciphertext[len(ENCRYPTED_PREFIX) :]
    try:
        packed = base64.b64decode(raw_b64)
    except Exception as exc:
        logger.error("Base64 decode failed for encrypted credential: %s", exc)
        return ""

    lib = _get_crypto_lib()
    if lib:
        try:
            decrypted = _decrypt_aes_gcm_native(lib, key, packed)
            return decrypted.decode("utf-8")
        except Exception:
            try:
                decrypted = _pure_python_ctr_hmac_decrypt(key, packed)
                return decrypted.decode("utf-8")
            except Exception as exc2:
                logger.error("Decryption failed: %s", exc2)
                return ""
    else:
        try:
            decrypted = _pure_python_ctr_hmac_decrypt(key, packed)
            return decrypted.decode("utf-8")
        except Exception as exc:
            logger.error("Fallback decryption failed: %s", exc)
            return ""


# ---------------------------------------------------------------------------
# App Access Password Hashing & Verification
# ---------------------------------------------------------------------------

def validate_app_password(password: str) -> Tuple[bool, str]:
    """
    Validates user-provided application access password.
    Requirements:
    - Minimum length of 6 characters.
    - Can be combination of numbers, letters, and symbols.
    """
    if not password:
        return False, "Password cannot be empty."
    if len(password) < 6:
        return False, "Password must be at least 6 characters long."
    return True, ""


def hash_app_password(password: str) -> str:
    """
    Hashes application password using PBKDF2-HMAC-SHA256 with 100,000 iterations and 16-byte random salt.
    Format: 'pbkdf2:sha256:100000:<salt_hex>:<hash_hex>'
    """
    salt = os.urandom(16)
    dk = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, PBKDF2_ROUNDS)
    return f"pbkdf2:sha256:{PBKDF2_ROUNDS}:{salt.hex()}:{dk.hex()}"


def verify_app_password(password: str, hashed: str) -> bool:
    """Verifies a password against the stored PBKDF2 hash."""
    if not password or not hashed:
        return False

    parts = hashed.split(":")
    if len(parts) != 5 or parts[0] != "pbkdf2" or parts[1] != "sha256":
        return False

    try:
        rounds = int(parts[2])
        salt = bytes.fromhex(parts[3])
        expected_dk = bytes.fromhex(parts[4])
        actual_dk = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, rounds)
        return hmac.compare_digest(actual_dk, expected_dk)
    except Exception:
        return False
