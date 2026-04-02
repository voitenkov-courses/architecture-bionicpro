ATTACH TABLE _ UUID 'fd7a4568-a787-45f4-8da5-059ea7de2e64'
(
    `user_id` UInt32,
    `prosthesis_type` String,
    `muscle_group` String,
    `signal_frequency` UInt32,
    `signal_duration` UInt32,
    `signal_amplitude` Decimal(5, 2),
    `signal_time` DateTime
)
ENGINE = MergeTree
ORDER BY (user_id, prosthesis_type, signal_time)
SETTINGS index_granularity = 8192
