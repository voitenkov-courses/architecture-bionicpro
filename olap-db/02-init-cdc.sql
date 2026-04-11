-- Приём данных через механизм KafkaEngine

CREATE TABLE IF NOT EXISTS crm_customers_queue (
    raw String
) ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:29092',
    kafka_topic_list = 'crm.public.customers',
    kafka_group_name = 'clickhouse-crm-cdc-v2',
    kafka_format = 'JSONAsString',
    kafka_max_block_size = 1048576,
    kafka_poll_timeout_ms = 1000;

CREATE TABLE IF NOT EXISTS crm_customers (
    id          UInt32,
    name        String,
    email       String
) ENGINE = ReplacingMergeTree(_ts)
ORDER BY id;

-- Создание MaterializedView для витрины в Clickhouse

CREATE MATERIALIZED VIEW IF NOT EXISTS crm_customers_mv TO crm_customers AS
SELECT
    coalesce(
        JSONExtractUInt(raw, 'after', 'id'),
        JSONExtractUInt(raw, 'before', 'id')
    ) AS id,
    JSONExtractString(raw, 'after', 'name')    AS name,
    JSONExtractString(raw, 'after', 'email')   AS email

FROM crm_customers_queue;

CREATE OR REPLACE VIEW user_reports_cdc AS
SELECT
    c.id                                        AS user_id,
    c.name                                      AS customer_name,
    c.email                                     AS customer_email,
    e.prosthesis.                               AS prosthesis,
    count()                                     AS total_signals,
    min(e.signal_time)                          AS min_signal_time,
    max(e.signal_time)                          AS max_signal_time,
    now()                                       AS report_updated
FROM (SELECT * FROM crm_customers FINAL) AS c
INNER JOIN emg_sensor_data AS e
    ON c.id = e.user_id
WHERE c._is_deleted = 0
GROUP BY
    c.id,
    c.name,
    c.email,
    e.prosthesis;