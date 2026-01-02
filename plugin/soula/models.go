package soula

import "gorm.io/gorm"

// Category 热搜分类
type Category struct {
	Name  string          `json:"name"`
	Alias string          `json:"alias"`
	Icon  string          `json:"icon"`
	Items []HotSearchItem `json:"items" gorm:"foreignKey:CategoryID"`

	gorm.Model
}

// HotSearchItem 热搜项
type HotSearchItem struct {
	CategoryID uint   `json:"category_id"`
	Term       string `json:"term"`
	Score      int    `json:"score"`

	gorm.Model
}

// CollectedResource 采集资源
type CollectedResource struct {
	UniqueID        string         `json:"unique_id" gorm:"uniqueIndex"`
	Channel         string         `json:"channel"`
	AiTitle         string         `json:"ai_title"`
	AiDescription   string         `json:"ai_description"`
	AiTags          string         `json:"ai_tags"` // JSON string or comma separated
	ImageUrl        string         `json:"image_url"`
	OriginalTitle   string         `json:"original_title"`
	OriginalContent string         `json:"original_content"`
	PanURL        string         `json:"pan_url"`
	PanPassword   string         `json:"pan_password"`
	Views           int            `json:"views"`
	Status          int            `json:"status"`
	StatusText      string         `json:"status_text"`
	Category        string         `json:"category"`
	Links           []ResourceLink `json:"links" gorm:"foreignKey:ResourceID"`

	gorm.Model
}

// ResourceLink 资源详情链接
type ResourceLink struct {
	ResourceID uint   `json:"resource_id"`
	Type       string `json:"type"` // e.g., "aliyun"
	URL        string `json:"url"`
	Password   string `json:"password"`
	Note       string `json:"note"`
	Datetime   string `json:"datetime"`
	Source     string `json:"source"`
	Image     string `json:"image"` // JSON string

	gorm.Model
}
