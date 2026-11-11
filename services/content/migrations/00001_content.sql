-- +goose Up
CREATE TABLE content_items (
  id CHAR(36) PRIMARY KEY,
  seed_key VARCHAR(64) NULL,
  type VARCHAR(32) NOT NULL,
  published_rev_id CHAR(36) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_seed_key (seed_key),
  INDEX idx_type (type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE content_revisions (
  id CHAR(36) PRIMARY KEY,
  item_id CHAR(36) NOT NULL,
  attrs LONGTEXT NOT NULL,
  texts LONGTEXT NOT NULL,
  created_by VARCHAR(64) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_item (item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- One open request (draft|pending) per item; approved|rejected are closed.
CREATE TABLE approval_requests (
  id CHAR(36) PRIMARY KEY,
  item_id CHAR(36) NOT NULL,
  rev_id CHAR(36) NOT NULL,
  status VARCHAR(16) NOT NULL,
  editors TEXT NOT NULL,
  submitted_by VARCHAR(64) NULL,
  reason TEXT NULL,
  decided_by VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  decided_at DATETIME(6) NULL,
  INDEX idx_item_status (item_id, status),
  INDEX idx_status (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE preview_tokens (
  token VARCHAR(64) PRIMARY KEY,
  item_id CHAR(36) NOT NULL,
  rev_id CHAR(36) NOT NULL,
  expires_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE audit_log (
  id CHAR(36) PRIMARY KEY,
  at DATETIME(6) NOT NULL,
  actor_id VARCHAR(64) NOT NULL,
  actor_roles VARCHAR(255) NOT NULL,
  action VARCHAR(64) NOT NULL,
  entity VARCHAR(64) NOT NULL,
  entity_id VARCHAR(64) NOT NULL,
  before_json LONGTEXT NULL,
  after_json LONGTEXT NULL,
  reason TEXT NULL,
  INDEX idx_entity (entity, entity_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE audit_log;
DROP TABLE preview_tokens;
DROP TABLE approval_requests;
DROP TABLE content_revisions;
DROP TABLE content_items;
