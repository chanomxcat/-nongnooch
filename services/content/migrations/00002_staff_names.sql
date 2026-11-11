-- +goose Up
CREATE TABLE staff_names (
  sub VARCHAR(64) PRIMARY KEY,
  name VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE staff_names;
