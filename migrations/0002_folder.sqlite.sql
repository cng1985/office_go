CREATE TABLE office_folder (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL,
    parent_id INTEGER NULL,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_office_folder_owner FOREIGN KEY (owner_id) REFERENCES account_user (id),
    CONSTRAINT fk_office_folder_parent FOREIGN KEY (parent_id) REFERENCES office_folder (id)
);

CREATE INDEX idx_office_folder_owner_parent ON office_folder (owner_id, parent_id, id);

ALTER TABLE office_document ADD COLUMN folder_id INTEGER NULL REFERENCES office_folder (id);

CREATE INDEX idx_office_document_owner_folder ON office_document (owner_id, folder_id, updated_at, id);
