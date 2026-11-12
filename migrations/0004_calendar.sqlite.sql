CREATE TABLE calendar_event (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    all_day INTEGER NOT NULL,
    starts_at DATETIME NOT NULL,
    ends_at DATETIME NOT NULL,
    start_on TEXT NOT NULL DEFAULT '',
    end_on TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_calendar_event_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_calendar_event_owner_time ON calendar_event (owner_id, starts_at, ends_at);

CREATE INDEX idx_calendar_event_owner_day ON calendar_event (owner_id, start_on, end_on);
