-- +goose Up
CREATE TABLE products (
  id CHAR(36) PRIMARY KEY, kind VARCHAR(10) NOT NULL, age_group VARCHAR(10) NOT NULL,
  names LONGTEXT NOT NULL, thai_satang BIGINT NULL, foreign_satang BIGINT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE quota (
  visit_date DATE PRIMARY KEY, total INT NOT NULL, held INT NOT NULL DEFAULT 0, sold INT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE orders (
  id CHAR(36) PRIMARY KEY, order_no VARCHAR(16) NOT NULL UNIQUE, member_id CHAR(36) NOT NULL,
  lang CHAR(2) NOT NULL, visit_date DATE NOT NULL, status VARCHAR(10) NOT NULL,
  total_satang BIGINT NOT NULL, day_tickets INT NOT NULL, hold_expires_at DATETIME(6) NOT NULL,
  paid_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL,
  INDEX idx_member (member_id, created_at), INDEX idx_hold (status, hold_expires_at), INDEX idx_paid (status, paid_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_items (
  id CHAR(36) PRIMARY KEY, order_id CHAR(36) NOT NULL, product_id CHAR(36) NOT NULL, kind VARCHAR(10) NOT NULL,
  audience VARCHAR(8) NOT NULL, price_satang BIGINT NOT NULL, holder_name VARCHAR(200) NOT NULL,
  holder_nationality CHAR(2) NOT NULL, holder_id_last4 CHAR(4) NOT NULL, thai_price BOOLEAN NOT NULL,
  INDEX idx_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE payments (
  id CHAR(36) PRIMARY KEY, order_id CHAR(36) NOT NULL, amount_satang BIGINT NOT NULL, method VARCHAR(12) NOT NULL,
  status VARCHAR(16) NOT NULL, kbank_payment_id VARCHAR(64) NULL, created_at DATETIME(6) NOT NULL, paid_at DATETIME(6) NULL,
  INDEX idx_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE webhook_events (
  event_id VARCHAR(64) PRIMARY KEY, received_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE idempotency (
  member_id CHAR(36) NOT NULL, endpoint VARCHAR(80) NOT NULL, idem_key VARCHAR(100) NOT NULL,
  status INT NOT NULL DEFAULT 0, body LONGTEXT NULL, created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (member_id, endpoint, idem_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_seq (day CHAR(6) PRIMARY KEY, n INT NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE order_seq; DROP TABLE idempotency; DROP TABLE webhook_events; DROP TABLE payments;
DROP TABLE order_items; DROP TABLE orders; DROP TABLE quota; DROP TABLE products;
