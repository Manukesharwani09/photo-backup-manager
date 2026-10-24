// Package entity holds one struct per database table (see migrations/). Entities carry data only;
// business rules live in internal/application and internal/planner, queries in internal/infrastructure/repository.
package entity

import "time"

// BaseModel holds the id and audit columns every table has.
type BaseModel struct {
	ID        int64     `db:"id" json:"id"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}
