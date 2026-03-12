-- Prism ClickHouse Schema

CREATE DATABASE IF NOT EXISTS prism;

-- Main spans table
CREATE TABLE IF NOT EXISTS prism.spans (
    trace_id         FixedString(32),
    span_id          FixedString(16),
    parent_span_id   String DEFAULT '',
    operation        LowCardinality(String),
    service          LowCardinality(String),
    kind             LowCardinality(String),
    start_us         UInt64,
    duration_us      UInt64,
    status           LowCardinality(String),
    tags             Map(String, String),
    events           String,
    date             Date DEFAULT toDate(fromUnixTimestamp64Micro(start_us))
)
ENGINE = MergeTree()
PARTITION BY date
ORDER BY (trace_id, start_us)
TTL date + INTERVAL 14 DAY
SETTINGS index_granularity = 8192;

-- Secondary indices
ALTER TABLE prism.spans ADD INDEX IF NOT EXISTS idx_service service TYPE set(100) GRANULARITY 4;
ALTER TABLE prism.spans ADD INDEX IF NOT EXISTS idx_operation operation TYPE tokenbf_v1(256, 2, 0) GRANULARITY 4;

-- Materialized view: service operations aggregate
CREATE MATERIALIZED VIEW IF NOT EXISTS prism.service_operations_mv
ENGINE = SummingMergeTree()
ORDER BY (service, operation, date)
AS SELECT
    service,
    operation,
    date,
    count()              AS call_count,
    countIf(status = 'error') AS error_count,
    sum(duration_us)     AS total_duration_us,
    max(duration_us)     AS max_duration_us
FROM prism.spans
GROUP BY service, operation, date;

-- Materialized view: service dependencies
CREATE MATERIALIZED VIEW IF NOT EXISTS prism.service_dependencies_mv
ENGINE = SummingMergeTree()
ORDER BY (parent_service, child_service, date)
AS SELECT
    s1.service AS parent_service,
    s2.service AS child_service,
    s2.date    AS date,
    count()    AS call_count,
    countIf(s2.status = 'error') AS error_count,
    avg(s2.duration_us) AS avg_duration_us
FROM prism.spans s1
INNER JOIN prism.spans s2
    ON s1.trace_id = s2.trace_id
    AND s1.span_id = s2.parent_span_id
WHERE s1.service != s2.service
GROUP BY parent_service, child_service, date;
