package soula

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"pansou/model"
	"pansou/plugin"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	// 鉴权中间件
	soula.Use(func(c *gin.Context) {
		// 优先从环境变量获取Token
		validToken := cmp.Or(os.Getenv("SOULA_TOKEN"), "soula-token-2026")
		token := c.GetHeader("X-Token")

		if token != validToken {
			c.JSON(401, gin.H{
				"code":    401,
				"message": "Unauthorized: Invalid or missing Soula Token",
			})
			c.Abort()
			return
		}
		c.Next()
	})

	soula.GET("/categories", sa.handleCategories)
	soula.GET("/collected-resources/random", sa.handleResourcesRandom)
	soula.GET("/collected-resources", sa.handleResources)
	soula.GET("/resource/:param", sa.handleResource)
	soula.GET("/collected-resources/hot", sa.handleDailyHotResources)

	fmt.Printf("[SOULA] Web路由已注册并启用鉴权: /api/...\n")
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

	items := make([]gin.H, 0)
	for _, res := range resources {
		items = append(items, gin.H{
			"id":        res.ID,
			"unique_id": res.UniqueID,
			"title":     res.Title,
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
	page = cmp.Or(page, 1)

	pageSizeStr := c.DefaultQuery("pageSize", "10")
	pageSize, _ := strconv.Atoi(pageSizeStr)
	pageSize = cmp.Or(pageSize, 10)

	// 2. Parse limits for items per category
	limitStr := c.DefaultQuery("limit", "24")
	limit, _ := strconv.Atoi(limitStr)
	limit = cmp.Or(limit, 24)

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
			"id":    cat.ID,
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

	items := make([]gin.H, 0)
	for _, resource := range resources {
		tags := make([]string, 0)
		json.Unmarshal([]byte(resource.Tags), &tags)

		item := gin.H{
			"id":               resource.ID,
			"unique_id":        resource.UniqueID,
			"channel":          resource.Channel,
			"title":            resource.Title,
			"description":      resource.Description,
			"tags":             tags,
			"image_url":        resource.ImageUrl,
			"original_title":   resource.OriginalTitle,
			"original_content": resource.OriginalContent,
			"pan_url":          resource.PanURL,
			"pan_password":     resource.PanPassword,
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
		mergedByType[link.Type] = append(mergedByType[link.Type], gin.H{
			"url":      link.URL,
			"password": link.Password,
			"note":     link.Note,
			"datetime": link.Datetime,
			"source":   link.Source,
			"image":    link.Image,
		})
	}

	// Parse tags for the main resource
	tags := make([]string, 0)
	json.Unmarshal([]byte(resource.Tags), &tags)

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":               resource.ID,
			"unique_id":        resource.UniqueID,
			"title":            resource.Title,
			"description":      resource.Description,
			"tags":             tags,
			"image_url":        resource.ImageUrl,
			"original_title":   resource.OriginalTitle,
			"original_content": resource.OriginalContent,
			"pan_url":          resource.PanURL,
			"pan_password":     resource.PanPassword,
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

	items := make([]gin.H, 0)
	for _, res := range resources {
		tags := make([]string, 0)
		json.Unmarshal([]byte(res.Tags), &tags)

		items = append(items, gin.H{
			"id":               res.ID,
			"unique_id":        res.UniqueID,
			"channel":          res.Channel,
			"title":            res.Title,
			"description":      res.Description,
			"tags":             tags,
			"image_url":        res.ImageUrl,
			"original_title":   res.OriginalTitle,
			"original_content": res.OriginalContent,
			"pan_url":          res.PanURL,
			"pan_password":     res.PanPassword,
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
