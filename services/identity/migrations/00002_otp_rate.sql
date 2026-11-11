-- +goose Up
CREATE TABLE otp_rate (
  k VARCHAR(300) PRIMARY KEY
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE otp_rate;
