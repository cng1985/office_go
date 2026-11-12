CREATE TABLE calendar_event (
    id BIGINT NOT NULL AUTO_INCREMENT,
    owner_id BIGINT NOT NULL,
    title VARCHAR(80) NOT NULL,
    all_day TINYINT(1) NOT NULL,
    starts_at DATETIME(3) NOT NULL,
    ends_at DATETIME(3) NOT NULL,
    start_on VARCHAR(10) NOT NULL DEFAULT '',
    end_on VARCHAR(10) NOT NULL DEFAULT '',
    location VARCHAR(80) NOT NULL DEFAULT '',
    notes VARCHAR(2000) NOT NULL DEFAULT '',
    color VARCHAR(7) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_calendar_event_owner_time (owner_id, starts_at, ends_at),
    KEY idx_calendar_event_owner_day (owner_id, start_on, end_on),
    CONSTRAINT fk_calendar_event_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
