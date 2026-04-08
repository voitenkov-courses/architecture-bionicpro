CREATE TABLE IF NOT EXISTS emg_sensor_data (
    user_id     UInt32,
    prosthesis  String,
    signal      UInt32,
    signal_time DateTime
) ENGINE = MergeTree()
ORDER BY (user_id, prosthesis, signal_time);

INSERT INTO emg_sensor_data (user_id, prothesis, signal, signal_time) VALUES
  (1,hand,3638,'2026-03-01 00:00:01'),
  (1,leg,1123,'2026-03-01 00:00:01'),
  (2,hand,4555,'2026-03-01 00:00:01'),
  (2,leg,6788,'2026-03-01 00:00:01'),
  (3,hand,4567,'2026-03-01 00:00:01'),
  (3,leg,7899,'2026-03-01 00:00:01');

CREATE TABLE IF NOT EXISTS user_reports (
    user_id          UInt32,
    customer_name    String,
    customer_email   String,
    prosthesis.      String,
    total_signals    UInt64,
    min_signal_time  DateTime,
    max_signal_time  DateTime,
    report_updated   DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (user_id, prosthesis_type);