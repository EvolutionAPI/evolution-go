package user_service

type SetProfilePictureResponse struct {
	Image string `json:"image"`
}

type SetProfileNameResponse struct {
	Name string `json:"name"`
}

type SetProfileStatusResponse struct {
	Status string `json:"status"`
}