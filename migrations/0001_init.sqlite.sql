CREATE TABLE account_user (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX uk_account_user_username ON account_user (username);

CREATE TABLE account_session (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    revoked_at DATETIME NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_account_session_user FOREIGN KEY (user_id) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_account_session_token ON account_session (token_hash);

CREATE INDEX idx_account_session_user ON account_session (user_id);

CREATE TABLE office_document (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    current_version_id INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT fk_office_document_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_office_document_owner_updated ON office_document (owner_id, updated_at, id);

CREATE INDEX idx_office_document_owner_kind ON office_document (owner_id, kind);

CREATE TABLE office_document_version (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL,
    version_no INTEGER NOT NULL,
    title TEXT NOT NULL,
    kind TEXT NOT NULL,
    content TEXT NOT NULL,
    content_sha TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT fk_office_document_version_document FOREIGN KEY (document_id) REFERENCES office_document (id),
    CONSTRAINT fk_office_document_version_author FOREIGN KEY (created_by) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_office_document_version_no ON office_document_version (document_id, version_no);

CREATE INDEX idx_office_document_version_created ON office_document_version (document_id, version_no);
