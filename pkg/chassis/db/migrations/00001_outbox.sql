-- +goose Up
CREATE TABLE outbox (
  id CHAR(36) PRIMARY KEY,
  type VARCHAR(64) NOT NULL,
  payload LONGTEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  INDEX idx_unpublished (published_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE processed_events (
  consumer VARCHAR(64) NOT NULL,
  event_id CHAR(36) NOT NULL,
  processed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (consumer, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox;
