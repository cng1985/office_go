CREATE TABLE account_team (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_account_team_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_account_team_owner ON account_team (owner_id, id);

CREATE TABLE account_team_member (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_account_team_member_team FOREIGN KEY (team_id) REFERENCES account_team (id),
    CONSTRAINT fk_account_team_member_user FOREIGN KEY (user_id) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_account_team_member ON account_team_member (team_id, user_id);

CREATE INDEX idx_account_team_member_user ON account_team_member (user_id, team_id);

ALTER TABLE office_document ADD COLUMN team_id INTEGER NULL REFERENCES account_team (id);

CREATE INDEX idx_office_document_team ON office_document (team_id, updated_at, id);

ALTER TABLE office_folder ADD COLUMN team_id INTEGER NULL REFERENCES account_team (id);

CREATE INDEX idx_office_folder_team ON office_folder (team_id, parent_id, id);
