CREATE TABLE account_user (
    id BIGINT NOT NULL AUTO_INCREMENT,
    username VARCHAR(32) NOT NULL,
    display_name VARCHAR(64) NOT NULL,
    password_hash VARCHAR(100) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_account_user_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE account_session (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    token_hash CHAR(64) NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    revoked_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_account_session_token (token_hash),
    KEY idx_account_session_user (user_id),
    CONSTRAINT fk_account_session_user FOREIGN KEY (user_id) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE office_document (
    id BIGINT NOT NULL AUTO_INCREMENT,
    owner_id BIGINT NOT NULL,
    kind VARCHAR(16) NOT NULL,
    title VARCHAR(200) NOT NULL,
    current_version_id BIGINT NOT NULL DEFAULT 0,
    revision BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_office_document_owner_updated (owner_id, updated_at, id),
    KEY idx_office_document_owner_kind (owner_id, kind),
    CONSTRAINT fk_office_document_owner FOREIGN KEY (owner_id) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE office_document_version (
    id BIGINT NOT NULL AUTO_INCREMENT,
    document_id BIGINT NOT NULL,
    version_no BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    content_sha CHAR(64) NOT NULL,
    byte_size BIGINT NOT NULL,
    note VARCHAR(200) NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_office_document_version_no (document_id, version_no),
    KEY idx_office_document_version_created (document_id, version_no),
    CONSTRAINT fk_office_document_version_document FOREIGN KEY (document_id) REFERENCES office_document (id),
    CONSTRAINT fk_office_document_version_author FOREIGN KEY (created_by) REFERENCES account_user (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
