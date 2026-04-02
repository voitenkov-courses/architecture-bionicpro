ATTACH TABLE _ UUID '66632e92-6060-43b4-83ed-a7d48cff0f88'
(
    `user_id` UInt32,
    `customer_name` String,
    `customer_email` String,
    `prosthesis_type` String,
    `total_signals` UInt64,
    `avg_amplitude` Float64,
    `avg_frequency` Float64,
    `avg_duration` Float64,
    `min_signal_time` DateTime,
    `max_signal_time` DateTime,
    `report_updated` DateTime DEFAULT now()
)
ENGINE = MergeTree
ORDER BY (user_id, prosthesis_type)
SETTINGS index_granularity = 8192
