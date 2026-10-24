package entity

// AppUser is an owner: the person whose Google accounts and phones are managed.
type AppUser struct {
	BaseModel
	Email string `db:"email" json:"email"`
}
