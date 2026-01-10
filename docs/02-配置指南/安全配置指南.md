# Week 2 Security Hardening Setup Guide

## Overview
This document outlines the security enhancements implemented during Week 2, focusing on encryption enforcement, audit logging reliability, and data safety.

## 1. Encryption Enforcement (Production Mode)
In `production` environment, the application now strictly enforces encryption:
- **Startup Check**: The application will fail to start if the encryption service cannot initialize (missing or invalid `ENCRYPTION_KEY`).
- **Write Protection**: Any attempt to save Bilibili account data will be rejected if encryption fails.
- **Legacy Prevention**: Plaintext fallback is disabled in production.

**Configuration:**
Ensure `ENVIRONMENT=production` in your `.env` file or environment variables.

## 2. Audit Log Reliability
The Audit Service has been enhanced for reliability:
- **Buffering**: Logs are buffered in memory and written to the database in batches.
- **Retry Mechanism**: Database writes use exponential backoff (retry up to 3 times) to handle transient failures.
- **Blocking**: When the buffer is full under high load, the logger will briefly block (with timeout) to prevent data loss, rather than immediately dropping logs.

## 3. Encrypted Database Backups
Automated encrypted backups are now enabled:
- **Schedule**: Every day at 04:00 AM.
- **Location**: `backups/` directory.
- **File Format**: `backup_YYYYMMDD_HHMMSS.enc`. these files are AES-GCM encrypted using the same system `ENCRYPTION_KEY`.
- **Retention**: Backups older than 7 days are automatically cleaned up.

**Restore Procedure:**
1. Decrypt the file using the `crypto` package tools (or a custom script with the correct key).
2. Replace the `.db` file (SQLite) with the decrypted backup.

## 4. API Security Enhancements
- **Encryption Version**: The Bilibili Account API (`/api/v1/bilibili/accounts`) now returns an `encryption_version` field.
  - `2`: Fully encrypted (Current Standard)
  - `1`: Legacy Encrypted
  - `0`/`null`: Plaintext (Should not exist in production)
- **CORS**: In Development mode, if `CORSAllowedOrigins` is configured in `config.toml`, it is now respected (previously defaulted to allow all).

## 5. Performance Optimization
- **Dirty Checking**: Encryption is now only performed when account data has actually changed (`Dirty` flag), reducing CPU usage during repetitive save operations.

## Checklist for Deployment
- [ ] Set `ENVIRONMENT=production`.
- [ ] Verify `ENCRYPTION_KEY` is set and valid (32 chars for AES-256).
- [ ] Check `logs/app.log` for successful "Encryption service initialized" message.
- [ ] Verify `backups/` directory exists and is writable.
