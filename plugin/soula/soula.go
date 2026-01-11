package soula

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"pansou/config"
	"pansou/model"
	"pansou/plugin"
	"path/filepath"
	"strconv"
	"sync"

	"gorm.io/gorm/logger"

	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type SoulaPlugin struct {
	*plugin.BaseAsyncPlugin

	mu          sync.RWMutex
	initialized bool     // 初始化状态标记
	DB          *gorm.DB // 数据库连接
}

func (sa *SoulaPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
	if keyword == "" || sa.DB == nil {
		return []model.SearchResult{}, nil
	}

	var resources []CollectedResource
	// 简单的模糊搜索
	searchTerm := "%" + keyword + "%"
	err := sa.DB.Where("title LIKE ? OR description LIKE ? OR original_content LIKE ?",
		searchTerm, searchTerm, searchTerm).
		Order("views desc").
		Limit(50).
		Find(&resources).Error

	if err != nil {
		return nil, err
	}

	results := make([]model.SearchResult, 0, len(resources))
	for _, res := range resources {
		results = append(results, model.SearchResult{
			UniqueID: res.UniqueID,
			Channel:  res.Channel,
			Title:    res.Title,
			Content:  res.Description,
			Datetime: time.Time(res.CreatedAt),
			Category: res.Category,
		})
	}

	return results, nil
}

// RecordSearch 记录搜索关键词热度，并根据搜索结果自动归类
func (sa *SoulaPlugin) RecordSearch(keyword string, results []model.SearchResult) {
	if keyword == "" || len(results) == 0 || sa.DB == nil {
		return
	}

	// 1. 统计结果中的分类分布
	categoryCounts := make(map[string]int)
	for _, res := range results {
		if res.Category != "" {
			categoryCounts[res.Category]++
		}
	}

	// 2. 匹配规则：选择结果中出现次数最多的分类作为该搜索词的分类
	targetAlias := "all"
	if len(categoryCounts) > 0 {
		maxCount := 0
		for alias, count := range categoryCounts {
			if count > maxCount {
				maxCount = count
				targetAlias = alias
			}
		}
	}

	// 3. 查找目标分类
	var category Category
	if err := sa.DB.Where("alias = ?", targetAlias).First(&category).Error; err != nil {
		// 如果匹配的分类不存在，则回退到 "全部"
		if err := sa.DB.Where("alias = ?", "all").First(&category).Error; err != nil {
			return
		}
	}

	// 4. 更新或创建热搜项
	var item HotSearchItem
	err := sa.DB.Where("category_id = ? AND term = ?", category.ID, keyword).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			newItem := HotSearchItem{
				CategoryID: category.ID,
				Term:       keyword,
				Score:      1,
			}
			sa.DB.Create(&newItem)
		}
		return
	}

	// 关键词已存在，得分加 1
	sa.DB.Model(&item).Update("score", gorm.Expr("score + ?", 1))
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

	// 初始化数据库 (MySQL)
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "root"
	}
	dbPass := os.Getenv("DB_PASS")
	if dbPass == "" {
		dbPass = "root"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "3306"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "soula"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&allowNativePasswords=true",
		dbUser, dbPass, dbHost, dbPort, dbName)

	gormConfig := &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Discard,
		NowFunc: func() time.Time {
			return time.Now().In(time.Local)
		},
		//DisableForeignKeyConstraintWhenMigrating: true,
	}
	if config.AppConfig.RunMode == gin.DebugMode {
		gormConfig.Logger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second, // 慢 SQL 阈值
				LogLevel:                  logger.Info, // 关键：日志级别
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		)
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return fmt.Errorf("连接数据库(MySQL)失败: %v", err)
	}

	// 自动迁移
	if err := db.Set("gorm:table_options",
		"ENGINE=InnoDB AUTO_INCREMENT=10000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").
		AutoMigrate(
			&Category{},
			&HotSearchItem{},
			&CollectedResource{},
			&ResourceLink{},
		); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	sa.DB = db
	sa.initialized = true

	// 初始化分类数据
	if err := sa.seedCategories(); err != nil {
		fmt.Printf("警告: 初始化分类数据失败: %v\n", err)
	}

	return nil
}

func (sa *SoulaPlugin) seedCategories() error {
	categories := []struct {
		Name  string
		Alias string
		Icon  string
	}{
		{"全部资源", "all", "all"},
		{"电影", "movie", "movie"},
		{"电视剧", "series", "series"},
		{"动漫", "anime", "anime"},
		{"综艺", "play", "play"},
		{"电子书", "ebook", "ebook"},
		{"游戏", "game", "game"},
		{"软件", "software", "software"},
		{"教程", "course", "course"},
		{"文档", "document", "document"},
		{"音乐", "music", "music"},
		{"源码", "code", "code"},
		{"福利", "welfare", "welfare"},
		{"其他", "other", "other"},
	}

	for _, c := range categories {
		err := sa.DB.Where(Category{Alias: c.Alias}).FirstOrCreate(&Category{
			Name:  c.Name,
			Alias: c.Alias,
			Icon:  c.Icon,
		}).Error
		if err != nil {
			return err
		}
	}
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

	pageSizeStr := c.DefaultQuery("pageSize", "100")
	pageSize, _ := strconv.Atoi(pageSizeStr)
	pageSize = cmp.Or(pageSize, 100)

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

	// 6. Get today's start time for counting updates
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var catList []gin.H
	for _, cat := range categories {
		var items []HotSearchItem
		// Fetch limited items for each category
		sa.DB.Where("category_id = ?", cat.ID).Order("score desc").Limit(limit).Find(&items)

		// Fetch today's update count
		var todayCount int64
		if cat.Alias == "all" {
			sa.DB.Model(&CollectedResource{}).Where("created_at >= ?", todayStart).Count(&todayCount)
		} else {
			sa.DB.Model(&CollectedResource{}).Where("category = ? AND created_at >= ?", cat.Alias, todayStart).Count(&todayCount)
		}

		itemsJson := make([]gin.H, 0)
		for _, item := range items {
			itemsJson = append(itemsJson, gin.H{
				"term":  item.Term,
				"score": item.Score,
			})
		}
		catList = append(catList, gin.H{
			"id":          cat.ID,
			"name":        cat.Name,
			"alias":       cat.Alias,
			"icon":        cat.Icon,
			"items":       itemsJson,
			"today_count": todayCount,
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
	// MySQL specific random order
	if err := sa.DB.Order("RAND()").Limit(pageSize).Find(&resources).Error; err != nil {
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
			"original_content": resource.OriginalContent,
			"quality":          resource.Quality,
			"year":             resource.Year,
			"views":            resource.Views,
			"status":           resource.Status,
			"category":         resource.Category,
			"created_at":       resource.CreatedAt,
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
		mergedByType[link.CloudType] = append(mergedByType[link.CloudType], gin.H{
			"url":      link.URL,
			"password": link.Password,
			"note":     link.Note,
			"source":   link.Source,
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
			"original_content": resource.OriginalContent,
			"quality":          resource.Quality,
			"year":             resource.Year,
			"views":            resource.Views,
			"category":         resource.Category,
			"created_at":       resource.CreatedAt,
			"total_links":      len(resource.Links),
			"merged_by_type":   mergedByType,
		},
	})
}

func (sa *SoulaPlugin) handleResources(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	category := c.Query("category")
	keyword := c.Query("keyword")

	query := sa.DB.Model(&CollectedResource{})
	if category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}

	if keyword != "" {
		searchTerm := "%" + keyword + "%"
		query = query.Where("(title LIKE ? OR description LIKE ? OR original_content LIKE ?)",
			searchTerm, searchTerm, searchTerm)
	}

	sort := c.DefaultQuery("sort", "latest")
	order := "created_at desc"
	if sort == "hot" {
		order = "views desc"
	}

	var total int64
	query.Count(&total)

	// Calculate today's total updates
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var todayTotal int64
	sa.DB.Model(&CollectedResource{}).Where("created_at >= ?", todayStart).Count(&todayTotal)

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
			"original_content": res.OriginalContent,
			"quality":          res.Quality,
			"year":             res.Year,
			"views":            res.Views,
			"status":           res.Status,
			"category":         res.Category,
			"created_at":       res.CreatedAt,
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
			"today_total":  todayTotal,
			"current_page": page,
			"last_page":    lastPage,
			"per_page":     perPage,
		},
	})
}
