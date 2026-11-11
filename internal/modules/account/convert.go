package account

func UserEntityToResponse(entity *User) *AccountResponse {
	return &AccountResponse{
		ID:          entity.ID,
		Username:    entity.Username,
		DisplayName: entity.DisplayName,
		CreatedAt:   entity.CreatedAt,
	}
}
