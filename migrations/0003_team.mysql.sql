CREATE TABLE account_team (
    id BIGINT NOT NULL AUTO_INCREMENT,
    owner_id BIGINT NOT NULL,
    name VARCHAR(40) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_account_team_owner (owner_id, id),
    CONSTRAINT fk_account_team_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE account_team_member (
    id BIGINT NOT NULL AUTO_INCREMENT,
    team_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role VARCHAR(16) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_account_team_member (team_id, user_id),
    KEY idx_account_team_member_user (user_id, team_id),
    CONSTRAINT fk_account_team_member_team FOREIGN KEY (team_id) REFERENCES account_team (id),
    CONSTRAINT fk_account_team_member_user FOREIGN KEY (user_id) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE office_document ADD COLUMN team_id BIGINT NULL;

ALTER TABLE office_document ADD CONSTRAINT fk_office_document_team FOREIGN KEY (team_id) REFERENCES account_team (id);

CREATE INDEX idx_office_document_team ON office_document (team_id, updated_at, id);

ALTER TABLE office_folder ADD COLUMN team_id BIGINT NULL;

ALTER TABLE office_folder ADD CONSTRAINT fk_office_folder_team FOREIGN KEY (team_id) REFERENCES account_team (id);

CREATE INDEX idx_office_folder_team ON office_folder (team_id, parent_id, id);
