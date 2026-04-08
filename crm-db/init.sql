CREATE TABLE IF NOT EXISTS customers (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(100),
    email   VARCHAR(100),
);

INSERT INTO customers (id, name, email) VALUES
  (1, 'jane.smith', 'Jane Smith'),
  (2, 'john.doe', 'John Doe'),
  (3, 'alex.johnson', 'Alex Johnson');
