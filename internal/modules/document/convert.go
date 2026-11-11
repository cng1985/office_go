package document

import "encoding/json"

func documentToSimple(row *detailRow) *DocumentSimple {
	return &DocumentSimple{
		ID:        row.ID,
		Kind:      row.Kind,
		Title:     row.Title,
		Revision:  row.Revision,
		VersionNo: row.VersionNo,
		UpdatedAt: row.UpdatedAt,
	}
}

func documentToResponse(row *detailRow) *DocumentResponse {
	return &DocumentResponse{
		ID:        row.ID,
		Kind:      row.Kind,
		Title:     row.Title,
		Revision:  row.Revision,
		VersionID: row.VersionID,
		VersionNo: row.VersionNo,
		Content:   json.RawMessage(row.Content),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func versionToSimple(entity *DocumentVersion) *DocumentVersionSimple {
	return &DocumentVersionSimple{
		ID:         entity.ID,
		DocumentID: entity.DocumentID,
		VersionNo:  entity.VersionNo,
		Title:      entity.Title,
		Note:       entity.Note,
		ByteSize:   entity.ByteSize,
		CreatedAt:  entity.CreatedAt,
	}
}

func versionToResponse(entity *DocumentVersion) *DocumentVersionResponse {
	return &DocumentVersionResponse{
		ID:         entity.ID,
		DocumentID: entity.DocumentID,
		VersionNo:  entity.VersionNo,
		Title:      entity.Title,
		Kind:       entity.Kind,
		Note:       entity.Note,
		Content:    json.RawMessage(entity.Content),
		ByteSize:   entity.ByteSize,
		CreatedAt:  entity.CreatedAt,
	}
}
