-- +goose Up
CREATE TABLE pongs (
  event_id CHAR(36) PRIMARY KEY,
  trace_id CHAR(32) NOT NULL,
  created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE pongs;
