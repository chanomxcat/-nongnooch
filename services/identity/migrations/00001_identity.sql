-- +goose Up
CREATE TABLE otp_requests (
  id CHAR(36) PRIMARY KEY, contact VARCHAR(254) NOT NULL, kind VARCHAR(8) NOT NULL,
  code_hmac BINARY(32) NOT NULL, lang CHAR(2) NOT NULL, ip VARBINARY(16) NOT NULL,
  attempts TINYINT NOT NULL DEFAULT 0, expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL,
  INDEX idx_contact_time (contact, created_at), INDEX idx_ip_time (ip, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE refresh_tokens (
  id CHAR(36) PRIMARY KEY, subject VARCHAR(254) NOT NULL, lang CHAR(2) NOT NULL DEFAULT 'en',
  token_hash BINARY(32) NOT NULL UNIQUE,
  expires_at DATETIME(6) NOT NULL, revoked_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL,
  INDEX idx_subject (subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE otp_requests;
