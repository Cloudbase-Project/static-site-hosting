package models

import (
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Array of Sites
type Sites []*Site

type Site struct {
	ID               uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	CreatedAt        time.Time      `                                                       json:"-"` // auto populated by gorm
	UpdatedAt        time.Time      `                                                       json:"-"` // auto populated by gorm
	DeletedAt        gorm.DeletedAt `gorm:"index"                                           json:"-"` // auto populated by gorm
	BuildStatus      string         `gorm:"default:'NotBuilt'"                              json:"buildStatus"`
	BuildFailReason  string         `                                                       json:"buildFailReason"`
	DeployStatus     string         `gorm:"default:'NotDeployed'"                           json:"deployStatus"`
	DeployFailReason string         `                                                       json:"deployFailReason"`
	LastAction       string         `gorm:"default:'Create'"                                json:"lastAction"`
	ConfigID         uuid.UUID
	Config           Config
}

func (f *Sites) ToJSON(w io.Writer) error {
	e := json.NewEncoder(w)
	return e.Encode(f)
}

func (f *Site) ToJSON(w io.Writer) error {
	e := json.NewEncoder(w)
	return e.Encode(f)
}
