package soula

import "gorm.io/gorm"

// Category 资源分类
type Category struct {
	Name  string          `json:"name" gorm:"uniqueIndex"`
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

// CollectedResource 采集到的网盘资源（主表）
type CollectedResource struct {
	UniqueID        string         `json:"unique_id" gorm:"uniqueIndex"` // e.g., tg_channelid_msgid
	Channel         string         `json:"channel" gorm:"index"`         // 来源频道ID/名称
	Title           string         `json:"title" gorm:"index"`           // 提取出的标题
	Description     string         `json:"description"`                  // 简洁介绍
	OriginalContent string         `json:"original_content"`             // 原始消息全文
	Tags            string         `json:"tags" gorm:"type:text"`        // 标签 (JSON array string)
	ImageUrl        string         `json:"image_url"`                    // 封面图
	Category        string         `json:"category" gorm:"index"`        // 资源分类 (e.g., Movie, App)
	Quality         string         `json:"quality" gorm:"index"`         // 画质 (4K, 1080P)
	Year            string         `json:"year"`                         // 年份
	Views           int            `json:"views" gorm:"default:0"`
	Status          int            `json:"status" gorm:"default:1"` // 1:可用, 0:失效
	Links           []ResourceLink `json:"links" gorm:"foreignKey:ResourceID"`

	gorm.Model
}

// ResourceLink 具体网盘链接（从表）
type ResourceLink struct {
	ResourceID uint   `json:"resource_id" gorm:"index"`
	CloudType  string `json:"cloud_type" gorm:"index"` // alipan, quark, baidu, uc, xunlei, 115, tianyi, pikpak
	URL        string `json:"url" gorm:"index"`
	Password   string `json:"password"`
	Note       string `json:"note"`   // 链接备注
	Source     string `json:"source"` // 来源频道
	Status     int    `json:"status" gorm:"default:1"`

	gorm.Model
}
