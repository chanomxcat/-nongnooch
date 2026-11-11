-- +goose Up
CREATE TABLE tickets (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NOT NULL,
  order_item_id CHAR(36) NOT NULL,
  member_id CHAR(36) NULL,
  kind VARCHAR(12) NOT NULL,
  visit_date DATE NULL,
  holder_name VARCHAR(200) NOT NULL,
  id_last4 CHAR(4) NOT NULL,
  status VARCHAR(12) NOT NULL DEFAULT 'active',
  used_at DATETIME(6) NULL,
  qr TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_item (order_item_id),
  INDEX idx_order (order_id),
  INDEX idx_member (member_id, created_at),
  INDEX idx_visit (visit_date, status),
  INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE scans (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  ticket_id CHAR(36) NULL,
  gate_id VARCHAR(40) NOT NULL,
  result VARCHAR(12) NOT NULL,
  method VARCHAR(6) NOT NULL,
  scanned_at DATETIME(6) NOT NULL,
  INDEX idx_time (scanned_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
