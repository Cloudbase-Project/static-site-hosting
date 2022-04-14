package services

import (
	"fmt"
	"log"

	"github.com/Cloudbase-Project/static-site-hosting/models"
	// "github.com/gofrs/uuid"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProxyService struct {
	db *gorm.DB
	l  *log.Logger
}

func NewProxyService(db *gorm.DB, l *log.Logger) *ProxyService {
	return &ProxyService{db: db, l: l}
}

func (ps *ProxyService) VerifySite(siteId string) (*models.Site, error) {
	var site models.Site
	uniqueId, err := uuid.Parse(siteId)
	if err != nil {
		fmt.Printf("err.Error(): %v\n", err.Error())
		return nil, err
	}
	if err := ps.db.Where(&models.Site{ID: uniqueId}).First(&site).Error; err != nil {
		return nil, err
	}
	return &site, nil

}
