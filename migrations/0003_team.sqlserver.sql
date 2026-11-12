CREATE TABLE account_team (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    name NVARCHAR(40) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    updated_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_account_team_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_account_team_owner ON account_team (owner_id, id);

CREATE TABLE account_team_member (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    team_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role NVARCHAR(16) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_account_team_member_team FOREIGN KEY (team_id) REFERENCES account_team (id),
    CONSTRAINT fk_account_team_member_user FOREIGN KEY (user_id) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_account_team_member ON account_team_member (team_id, user_id);

CREATE INDEX idx_account_team_member_user ON account_team_member (user_id, team_id);

ALTER TABLE office_document ADD team_id BIGINT NULL;

ALTER TABLE office_document ADD CONSTRAINT fk_office_document_team FOREIGN KEY (team_id) REFERENCES account_team (id);

CREATE INDEX idx_office_document_team ON office_document (team_id, updated_at, id);

ALTER TABLE office_folder ADD team_id BIGINT NULL;

ALTER TABLE office_folder ADD CONSTRAINT fk_office_folder_team FOREIGN KEY (team_id) REFERENCES account_team (id);

CREATE INDEX idx_office_folder_team ON office_folder (team_id, parent_id, id);
