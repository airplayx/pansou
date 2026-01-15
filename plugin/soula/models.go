package soula

import (
	"gorm.io/gorm"
)

// Category 资源分类
type Category struct {
	ID    uint            `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name  string          `gorm:"column:name;type:varchar(64);uniqueIndex;not null" json:"name"`
	Alias string          `gorm:"column:alias;type:varchar(64);index" json:"alias"`
	Icon  string          `gorm:"column:icon;type:varchar(128)" json:"icon"`
	Items []HotSearchItem `json:"items" gorm:"foreignKey:CategoryID"`

	CreatedAt Timestamp      `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt Timestamp      `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// HotSearchItem 热搜项
type HotSearchItem struct {
	ID         uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CategoryID uint   `gorm:"column:category_id;not null;index" json:"category_id"`
	Term       string `gorm:"column:term;type:varchar(128);not null;index" json:"term"`
	Score      int    `gorm:"column:score;type:int(11);default:0" json:"score"`

	CreatedAt Timestamp      `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt Timestamp      `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// CollectedResource 采集到的网盘资源（主表）
type CollectedResource struct {
	ID              uint           `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UniqueID        string         `gorm:"column:unique_id;type:varchar(128);uniqueIndex;not null" json:"unique_id"`
	Channel         string         `gorm:"column:channel;type:varchar(128);index" json:"channel"`
	Title           string         `gorm:"column:title;type:varchar(1000);not null" json:"title"`
	Description     string         `gorm:"column:description;type:text" json:"description"`
	OriginalContent string         `gorm:"column:original_content;type:longtext" json:"original_content"`
	Tags            string         `gorm:"column:tags;type:text" json:"tags"`
	ImageUrl        string         `gorm:"column:image_url;type:text" json:"image_url"`
	Category        string         `gorm:"column:category;type:varchar(64);index" json:"category"`
	Quality         string         `gorm:"column:quality;type:varchar(32);index" json:"quality"`
	Year            string         `gorm:"column:year;type:varchar(16);index" json:"year"`
	Views           int            `gorm:"column:views;type:int(11);default:0" json:"views"`
	Status          int            `gorm:"column:status;type:tinyint(4);default:1" json:"status"`
	Links           []ResourceLink `json:"links" gorm:"foreignKey:ResourceID"`

	CreatedAt Timestamp      `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt Timestamp      `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// ResourceLink 具体网盘链接（从表）
type ResourceLink struct {
	ID         uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ResourceID uint   `gorm:"column:resource_id;not null;index" json:"resource_id"`
	CloudType  string `gorm:"column:cloud_type;type:varchar(32);index" json:"cloud_type"`
	URL        string `gorm:"column:url;type:varchar(1024);not null" json:"url"`
	Password   string `gorm:"column:password;type:varchar(64);default:''" json:"password"`
	Note       string `gorm:"column:note;type:varchar(255)" json:"note"`
	Source     string `gorm:"column:source;type:varchar(128)" json:"source"`
	Status     int    `gorm:"column:status;type:tinyint(4);default:1" json:"status"`

	CreatedAt Timestamp      `gorm:"column:created_at;type:datetime" json:"created_at"`
	UpdatedAt Timestamp      `gorm:"column:updated_at;type:datetime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}
