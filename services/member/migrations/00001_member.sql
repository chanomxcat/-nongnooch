-- +goose Up
CREATE TABLE members (
  id CHAR(36) PRIMARY KEY, phone VARCHAR(20) NULL UNIQUE, email VARCHAR(254) NULL UNIQUE,
  first_name VARCHAR(100) NOT NULL, last_name VARCHAR(100) NOT NULL, nationality CHAR(2) NOT NULL,
  id_kind VARCHAR(10) NOT NULL, id_country CHAR(2) NOT NULL, id_enc VARBINARY(128) NULL,
  id_hmac BINARY(32) NULL UNIQUE, id_last4 CHAR(4) NOT NULL, thai_verified BOOLEAN NOT NULL,
  lang CHAR(2) NOT NULL, channel VARCHAR(8) NOT NULL,
  invite_code VARCHAR(32) NULL, guide_id CHAR(36) NULL, company_id CHAR(36) NULL, invite_bound_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE consent_log (
  id CHAR(36) PRIMARY KEY, member_id CHAR(36) NOT NULL, kind VARCHAR(10) NOT NULL,
  granted BOOLEAN NOT NULL, version VARCHAR(16) NOT NULL, channel VARCHAR(8) NOT NULL,
  ip VARCHAR(45) NULL, created_at DATETIME(6) NOT NULL, INDEX idx_member_kind (member_id, kind, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE point_ledger (
  id CHAR(36) PRIMARY KEY, member_id CHAR(36) NOT NULL, delta_points INT NOT NULL,
  reason VARCHAR(16) NOT NULL, status VARCHAR(10) NOT NULL,
  ref_kind VARCHAR(16) NOT NULL, ref_id CHAR(36) NOT NULL, guide_id CHAR(36) NULL, note VARCHAR(500) NULL, actor VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_ref (ref_kind, ref_id, reason), INDEX idx_member_time (member_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose StatementBegin
CREATE TRIGGER point_ledger_no_update BEFORE UPDATE ON point_ledger FOR EACH ROW
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'point_ledger is append-only';
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER point_ledger_no_delete BEFORE DELETE ON point_ledger FOR EACH ROW
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'point_ledger is append-only';
-- +goose StatementEnd

-- Demo stand-in for AgentService.ValidateInvite (no agent service in the demo slice).
CREATE TABLE demo_invite_codes (
  code VARCHAR(32) PRIMARY KEY, guide_id CHAR(36) NOT NULL, company_id CHAR(36) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE demo_invite_codes;
DROP TABLE point_ledger;
DROP TABLE consent_log;
DROP TABLE members;
