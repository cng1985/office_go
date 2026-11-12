CREATE TABLE office_folder (
    id BIGINT NOT NULL AUTO_INCREMENT,
    owner_id BIGINT NOT NULL,
    parent_id BIGINT NULL,
    name VARCHAR(80) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_office_folder_owner_parent (owner_id, parent_id, id),
    CONSTRAINT fk_office_folder_owner FOREIGN KEY (owner_id) REFERENCES account_user (id),
    CONSTRAINT fk_office_folder_parent FOREIGN KEY (parent_id) REFERENCES office_folder (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE office_document ADD COLUMN folder_id BIGINT NULL;

ALTER TABLE office_document ADD CONSTRAINT fk_office_document_folder FOREIGN KEY (folder_id) REFERENCES office_folder (id);

CREATE INDEX idx_office_document_owner_folder ON office_document (owner_id, folder_id, updated_at, id);
