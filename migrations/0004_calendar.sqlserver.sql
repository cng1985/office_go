CREATE TABLE calendar_event (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    title NVARCHAR(80) NOT NULL,
    all_day BIT NOT NULL,
    starts_at DATETIMEOFFSET NOT NULL,
    ends_at DATETIMEOFFSET NOT NULL,
    start_on NVARCHAR(10) NOT NULL CONSTRAINT df_calendar_event_start_on DEFAULT '',
    end_on NVARCHAR(10) NOT NULL CONSTRAINT df_calendar_event_end_on DEFAULT '',
    location NVARCHAR(80) NOT NULL CONSTRAINT df_calendar_event_location DEFAULT '',
    notes NVARCHAR(2000) NOT NULL CONSTRAINT df_calendar_event_notes DEFAULT '',
    color NVARCHAR(7) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    updated_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_calendar_event_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_calendar_event_owner_time ON calendar_event (owner_id, starts_at, ends_at);

CREATE INDEX idx_calendar_event_owner_day ON calendar_event (owner_id, start_on, end_on);
