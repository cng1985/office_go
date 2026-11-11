CREATE TABLE account_user (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    username NVARCHAR(32) NOT NULL,
    display_name NVARCHAR(64) NOT NULL,
    password_hash NVARCHAR(100) NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    updated_at DATETIMEOFFSET NOT NULL
);

CREATE UNIQUE INDEX uk_account_user_username ON account_user (username);

CREATE TABLE account_session (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token_hash NVARCHAR(64) NOT NULL,
    expires_at DATETIMEOFFSET NOT NULL,
    revoked_at DATETIMEOFFSET NULL,
    created_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_account_session_user FOREIGN KEY (user_id) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_account_session_token ON account_session (token_hash);

CREATE INDEX idx_account_session_user ON account_session (user_id);

CREATE TABLE office_document (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    owner_id BIGINT NOT NULL,
    kind NVARCHAR(16) NOT NULL,
    title NVARCHAR(200) NOT NULL,
    current_version_id BIGINT NOT NULL CONSTRAINT df_office_document_current_version DEFAULT 0,
    revision BIGINT NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    updated_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_office_document_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
);

CREATE INDEX idx_office_document_owner_updated ON office_document (owner_id, updated_at, id);

CREATE INDEX idx_office_document_owner_kind ON office_document (owner_id, kind);

CREATE TABLE office_document_version (
    id BIGINT IDENTITY(1,1) NOT NULL PRIMARY KEY,
    document_id BIGINT NOT NULL,
    version_no BIGINT NOT NULL,
    title NVARCHAR(200) NOT NULL,
    kind NVARCHAR(16) NOT NULL,
    content NVARCHAR(MAX) NOT NULL,
    content_sha NVARCHAR(64) NOT NULL,
    byte_size BIGINT NOT NULL,
    note NVARCHAR(200) NOT NULL CONSTRAINT df_office_document_version_note DEFAULT N'',
    created_by BIGINT NOT NULL,
    created_at DATETIMEOFFSET NOT NULL,
    CONSTRAINT fk_office_document_version_document FOREIGN KEY (document_id) REFERENCES office_document (id),
    CONSTRAINT fk_office_document_version_author FOREIGN KEY (created_by) REFERENCES account_user (id)
);

CREATE UNIQUE INDEX uk_office_document_version_no ON office_document_version (document_id, version_no);

CREATE INDEX idx_office_document_version_created ON office_document_version (document_id, version_no);
