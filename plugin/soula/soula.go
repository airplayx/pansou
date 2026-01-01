package soula

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"pansou/model"
	"pansou/plugin"
	"path/filepath"
	"strconv"
	"sync"
)

type SoulaPlugin struct {
	*plugin.BaseAsyncPlugin

	mu          sync.RWMutex
	initialized bool     // 初始化状态标记
	DB          *gorm.DB // 数据库连接
}

func (sa *SoulaPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	//TODO implement me

	return []model.SearchResult{}, nil
}

// 存储目录
var StorageDir string

func init() {
	p := &SoulaPlugin{
		BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("soula", 1),
	}

	plugin.RegisterGlobalPlugin(p)
}

// Initialize 实现 InitializablePlugin 接口，延迟初始化插件
func (sa *SoulaPlugin) Initialize() error {
	if sa.initialized {
		return nil
	}

	// 初始化存储目录路径
	cachePath := os.Getenv("CACHE_PATH")
	if cachePath == "" {
		cachePath = "./cache"
	}
	StorageDir = filepath.Join(cachePath, "soula")

	// 初始化存储目录
	if err := os.MkdirAll(StorageDir, 0755); err != nil {
		return fmt.Errorf("创建存储目录失败: %v", err)
	}

	// 初始化数据库
	dbPath := filepath.Join(StorageDir, "soula.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&Category{}, &HotSearchItem{}, &CollectedResource{}, &ResourceLink{}); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	sa.DB = db
	sa.initialized = true
	return nil
}

// RegisterWebRoutes 注册Web路由
func (sa *SoulaPlugin) RegisterWebRoutes(router *gin.RouterGroup) {
	soula := router.Group("/api")
	soula.GET("/categories", sa.handleCategories)
	soula.GET("/collected-resources/random", sa.handleResourcesRandom)
	soula.GET("/collected-resources", sa.handleResources)
	soula.GET("/resource/:param", sa.handleResource)
	soula.GET("/collected-resources/hot", sa.handleDailyHotResources)

	fmt.Printf("[SOULA] Web路由已注册: /api/:param\n")
}

func (sa *SoulaPlugin) handleDailyHotResources(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit < 1 {
		limit = 10
	}

	// Logic: Get top viewed resources created in the last 7 days (to ensure data availability)
	// If you strictly want TODAY, use time.Now().Truncate(24 * time.Hour)
	// But 7 days is a safer "Trending" window for typical sites
	// daysAgo := time.Now().AddDate(0, 0, -7)

	var resources []CollectedResource
	// For "Today's Hot", we can just order by Views descending.
	// Optionally add a where clause like: .Where("created_at > ?", daysAgo)
	// For now, let's just show the global most popular to ensure we have data on the UI
	if err := sa.DB.Order("views desc").Limit(limit).Find(&resources).Error; err != nil {
		c.JSON(200, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"items": []gin.H{},
			},
		})
		return
	}

	var items []gin.H
	for _, res := range resources {
		items = append(items, gin.H{
			"id":        res.ID,
			"unique_id": res.UniqueID,
			"title":     res.AiTitle,
			"views":     res.Views,
			"category":  res.Category,
		})
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": items,
		},
	})
}

func (sa *SoulaPlugin) handleCategories(c *gin.Context) {
	// 1. Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	pageSizeStr := c.DefaultQuery("pageSize", "5")
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = 5
	}

	// 2. Parse limits for items per category
	limitStr := c.DefaultQuery("limit", "24")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 24
	}

	// 3. Count total categories
	var total int64
	sa.DB.Model(&Category{}).Count(&total)

	// 4. Calculate pagination
	offset := (page - 1) * pageSize
	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	// 5. Fetch categories for current page
	var categories []Category
	if err := sa.DB.Limit(pageSize).Offset(offset).Find(&categories).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "Failed to fetch categories"})
		return
	}

	var catList []gin.H
	for _, cat := range categories {
		var items []HotSearchItem
		// Fetch limited items for each category
		sa.DB.Where("category_id = ?", cat.ID).Order("score desc").Limit(limit).Find(&items)

		itemsJson := make([]gin.H, 0)
		for _, item := range items {
			itemsJson = append(itemsJson, gin.H{
				"term":  item.Term,
				"score": item.Score,
			})
		}
		catList = append(catList, gin.H{
			"name":  cat.Name,
			"icon":  cat.Icon,
			"items": itemsJson,
		})
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"categories":  catList,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"total_pages": totalPages,
		},
	})
}

func (sa *SoulaPlugin) handleResourcesRandom(c *gin.Context) {
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "6"))
	if pageSize < 1 {
		pageSize = 6
	}

	var resources []CollectedResource
	// SQLite specific random order
	if err := sa.DB.Order("RANDOM()").Limit(pageSize).Find(&resources).Error; err != nil {
		c.JSON(200, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"items": []gin.H{},
			},
		})
		return
	}

	var items []gin.H
	for _, resource := range resources {
		var tags []string
		json.Unmarshal([]byte(resource.AiTags), &tags)

		item := gin.H{
			"id":               resource.ID,
			"unique_id":        resource.UniqueID,
			"channel":          resource.Channel,
			"ai_title":         resource.AiTitle,
			"ai_description":   resource.AiDescription,
			"ai_tags":          tags,
			"image_urls":       resource.ImageUrl,
			"original_title":   resource.OriginalTitle,
			"original_content": resource.OriginalContent,
			"my_pan_url":       resource.MyPanURL,
			"my_pan_password":  resource.MyPanPassword,
			"views":            resource.Views,
			"status":           resource.Status,
			"status_text":      resource.StatusText,
			"category":         resource.Category,
			"created_at":       resource.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		items = append(items, item)
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": items,
		},
	})
}

func (sa *SoulaPlugin) handleResource(c *gin.Context) {
	uniqueID := c.Param("param")
	var resource CollectedResource
	if err := sa.DB.Preload("Links").Where("unique_id = ?", uniqueID).First(&resource).Error; err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "Resource not found"})
		return
	}

	// Increment views
	sa.DB.Model(&resource).UpdateColumn("views", gorm.Expr("views + ?", 1))

	mergedByType := make(map[string][]gin.H)
	for _, link := range resource.Links {
		var images []string
		json.Unmarshal([]byte(link.Images), &images)

		mergedByType[link.Type] = append(mergedByType[link.Type], gin.H{
			"url":      link.URL,
			"password": link.Password,
			"note":     link.Note,
			"datetime": link.Datetime,
			"source":   link.Source,
			"images":   images,
		})
	}

	// Parse tags and images for the main resource
	var tags []string
	json.Unmarshal([]byte(resource.AiTags), &tags)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":               resource.ID,
			"unique_id":        resource.UniqueID,
			"ai_title":         resource.AiTitle,
			"ai_description":   resource.AiDescription,
			"ai_tags":          tags,
			"image_url":        resource.ImageUrl,
			"original_title":   resource.OriginalTitle,
			"original_content": resource.OriginalContent,
			"my_pan_url":       resource.MyPanURL,
			"my_pan_password":  resource.MyPanPassword,
			"views":            resource.Views,
			"category":         resource.Category,
			"created_at":       resource.CreatedAt.Format("2006-01-02 15:04:05"),
			"total_links":      len(resource.Links),
			"merged_by_type":   mergedByType,
		},
	})
}

func (sa *SoulaPlugin) handleResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	category := c.Query("category")

	query := sa.DB.Model(&CollectedResource{})
	if category != "" {
		query = query.Where("category = ?", category)
	}

	sort := c.DefaultQuery("sort", "latest")
	order := "created_at desc"
	if sort == "hot" {
		order = "views desc"
	}

	var total int64
	query.Count(&total)

	var resources []CollectedResource
	offset := (page - 1) * perPage
	query.Order(order).Offset(offset).Limit(perPage).Find(&resources)

	var items []gin.H
	for _, res := range resources {
		var tags []string
		json.Unmarshal([]byte(res.AiTags), &tags)

		items = append(items, gin.H{
			"id":               res.ID,
			"unique_id":        res.UniqueID,
			"channel":          res.Channel,
			"ai_title":         res.AiTitle,
			"ai_description":   res.AiDescription,
			"ai_tags":          tags,
			"image_url":        res.ImageUrl,
			"original_title":   res.OriginalTitle,
			"original_content": res.OriginalContent,
			"my_pan_url":       res.MyPanURL,
			"my_pan_password":  res.MyPanPassword,
			"views":            res.Views,
			"status":           res.Status,
			"status_text":      res.StatusText,
			"category":         res.Category,
			"created_at":       res.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	lastPage := int(total) / perPage
	if int(total)%perPage != 0 {
		lastPage++
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items":        items,
			"total":        total,
			"current_page": page,
			"last_page":    lastPage,
			"per_page":     perPage,
		},
	})
}
