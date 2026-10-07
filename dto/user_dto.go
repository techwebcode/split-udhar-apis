package dto

type UpdateProfileRequest struct {
	FullName string `json:"full_name" binding:"omitempty,min=2,max=100"`
	Mobile   string `json:"mobile" binding:"omitempty"`
}


type ProfileImageResponse struct {
	ProfileImageKey string `json:"profile_image_key"`
	ProfileImageURL string `json:"profile_image_url"`
}
