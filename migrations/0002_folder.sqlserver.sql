CREATE TABLE office_folder (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    name NVARCHAR(80) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    updated_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_office_folder_owner FOREIGN KEY (owner_id) REFERENCES account_user (id),
    CONSTRAINT fk_office_folder_parent FOREIGN KEY (parent_id) REFERENCES office_folder (id)
);

CREATE INDEX idx_office_folder_owner_parent ON office_folder (owner_id, parent_id, id);

ALTER TABLE office_document ADD folder_id BIGINT NULL;

ALTER TABLE office_document ADD CONSTRAINT fk_office_document_folder FOREIGN KEY (folder_id) REFERENCES office_folder (id);

CREATE INDEX idx_office_document_owner_folder ON office_document (owner_id, folder_id, updated_at, id);
