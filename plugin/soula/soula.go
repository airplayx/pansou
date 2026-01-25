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
	"strings"
	"sync"

	"gorm.io/gorm/logger"

	"time"

	"math/rand"

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

func (sa *SoulaPlugin) Search(keyword string, ext map[string]any) ([]model.SearchResult, error) {
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

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

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
	if err = db.Set("gorm:table_options",
		"ENGINE=InnoDB AUTO_INCREMENT=10000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci").
		AutoMigrate(
			&Category{},
			&HotSearchItem{},
			&CollectedResource{},
			&ResourceLink{},
			&FriendLink{},
		); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	sa.DB = db
	sa.initialized = true

	// 初始化分类数据
	if err := sa.seedCategories(); err != nil {
		fmt.Printf("警告: 初始化分类数据失败: %v\n", err)
	}

	// 初始化友情链接数据
	if err := sa.seedFriendLinks(); err != nil {
		fmt.Printf("警告: 初始化友情链接数据失败: %v\n", err)
	}

	return nil
}

func (sa *SoulaPlugin) seedFriendLinks() error {
	var count int64
	sa.DB.Model(&FriendLink{}).Count(&count)
	if count > 0 {
		return nil
	}

	links := []FriendLink{
		{Name: "盘搜", URL: "https://pansou.cn", Description: "极简单的网盘搜索", Sort: 1, Category: "搜索"},
		{Name: "苏拉搜索", URL: "https://soula.io", Description: "专业网盘搜索引擎", Sort: 2, Category: "搜索"},
	}

	for _, l := range links {
		sa.DB.Create(&l)
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
		{"剧集", "series", "series"},
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
		// Use Assign to update Name even if record already exists
		err := sa.DB.Where(Category{Alias: c.Alias}).Assign(Category{Name: c.Name, Icon: c.Icon}).FirstOrCreate(&Category{}).Error
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
	soula.GET("/friend-links", sa.handleFriendLinks)
	soula.POST("/friend-links", sa.handleUpsertFriendLink)

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

	// 2. Parse search parameters
	keyword := c.Query("keyword")

	// 3. Parse limits for items per category
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

	// 6. Get today's start time for counting updates from query parameter
	todayStartStr := c.Query("todayStart")
	todayStart, err := strconv.ParseInt(todayStartStr, 10, 64)
	if err != nil || todayStart == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "todayStart parameter is required"})
		return
	}

	var catList []gin.H
	for _, cat := range categories {
		var items []HotSearchItem
		// Fetch limited items for each category
		sa.DB.Where("category_id = ?", cat.ID).Order("score desc").Limit(limit).Find(&items)

		// Fetch today's update count or keyword count
		var todayCount int64
		if keyword != "" {
			searchTerm := "%" + keyword + "%"
			itemQuery := sa.DB.Model(&CollectedResource{})
			if cat.Alias != "all" {
				itemQuery = itemQuery.Where("category = ?", cat.Alias)
			}
			itemQuery.Where("(title LIKE ? OR description LIKE ? OR original_content LIKE ?)",
				searchTerm, searchTerm, searchTerm).Count(&todayCount)
		} else {
			// Using CONVERT_TZ for accurate timezone-based counts
			// Assuming created_at is stored in UTC or Database local time
			// Here we calculate the start of today in the user's timezone and compare
			if cat.Alias == "all" {
				sa.DB.Model(&CollectedResource{}).Where("created_at >= FROM_UNIXTIME(?)", todayStart).Count(&todayCount)
			} else {
				sa.DB.Model(&CollectedResource{}).Where("category = ? AND created_at >= FROM_UNIXTIME(?)", cat.Alias, todayStart).Count(&todayCount)
			}
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

	startTimeStr := c.Query("startTime")
	db := sa.DB.Model(&CollectedResource{})

	if startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			// Expecting timestamp in seconds
			db = db.Where("created_at >= FROM_UNIXTIME(?)", ts)
		}
	}

	var resources []CollectedResource
	// MySQL specific random order
	if err := db.Order("RAND()").Limit(pageSize).Find(&resources).Error; err != nil {
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

	// 计算增加的浏览量：10-100 之间，距离上次更新时间越久，增加量越大
	lastUpdate := time.Time(resource.UpdatedAt)
	if lastUpdate.IsZero() {
		lastUpdate = time.Time(resource.CreatedAt)
	}

	hoursDiff := time.Since(lastUpdate).Hours()
	if hoursDiff < 0 {
		hoursDiff = 0
	}

	// 以 24 小时为一个周期，达到最大偏移系数
	timeFactor := hoursDiff / 24.0
	if timeFactor > 1.0 {
		timeFactor = 1.0
	}

	// 基础增加 10，最大增加到 100
	// 随机范围随着时间拉大
	minInc := 10 + int(70*timeFactor) // 10 -> 80
	maxInc := 20 + int(80*timeFactor) // 20 -> 100

	increment := minInc
	if maxInc > minInc {
		increment += rand.Intn(maxInc - minInc + 1)
	}

	// 更新浏览量并同步更新时间，以便下次计算
	sa.DB.Model(&resource).Updates(map[string]interface{}{
		"views":      gorm.Expr("views + ?", increment),
		"updated_at": time.Now(),
	})

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
	perPage, _ := strconv.Atoi(c.DefaultQuery("perPage", "10"))
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
	todayStartStr := c.Query("todayStart")
	todayStart, err := strconv.ParseInt(todayStartStr, 10, 64)
	if err != nil || todayStart == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "todayStart parameter is required"})
		return
	}
	var todayTotal int64
	sa.DB.Model(&CollectedResource{}).Where("created_at >= FROM_UNIXTIME(?)", todayStart).Count(&todayTotal)

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

func normalizeFriendLinkURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "//")
	u = strings.TrimSuffix(u, "/")
	return strings.ToLower(u)
}

func (sa *SoulaPlugin) handleFriendLinks(c *gin.Context) {
	referer := c.Request.Referer()

	var links []FriendLink
	// 获取所有启用的友链，以便在内存中进行 Referer 匹配
	if err := sa.DB.Where("status = ?", 1).Order("sort asc, id asc").Find(&links).Error; err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "Failed to fetch friend links"})
		return
	}

	if referer != "" {
		normRef := normalizeFriendLinkURL(referer)
		foundIdx := -1
		for i, l := range links {
			normLink := normalizeFriendLinkURL(l.URL)
			// 如果 Referer 匹配该友链（例如来自该域名的某个页面），则认为命中
			if strings.HasPrefix(normRef, normLink) {
				foundIdx = i
				break
			}
		}

		// 如果找到了匹配项且不在第一位，则移动到第一位
		if foundIdx > 0 {
			match := links[foundIdx]
			// 从原切片中移除
			links = append(links[:foundIdx], links[foundIdx+1:]...)
			// 插入到开头
			links = append([]FriendLink{match}, links...)
		}
	}

	// 最终展示数量控制
	all := c.Query("all")
	if all != "true" && len(links) > 8 {
		links = links[:8]
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": links,
		},
	})
}

func (sa *SoulaPlugin) handleUpsertFriendLink(c *gin.Context) {
	var req struct {
		ID          uint   `json:"id"`
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		Category    string `json:"category" binding:"required"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
		Sort        int    `json:"sort"`
		Status      int    `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "Invalid parameters: " + err.Error()})
		return
	}

	link := FriendLink{
		Name:        req.Name,
		URL:         req.URL,
		Category:    req.Category,
		Icon:        req.Icon,
		Description: req.Description,
		Sort:        req.Sort,
		Status:      req.Status,
	}

	if req.ID > 0 {
		link.ID = req.ID
		if err := sa.DB.Model(&FriendLink{}).Where("id = ?", req.ID).Updates(&link).Error; err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "Failed to update friend link"})
			return
		}
	} else {
		// 检查重复
		var existingLinks []FriendLink
		sa.DB.Select("url").Find(&existingLinks)
		normalizedNew := normalizeFriendLinkURL(req.URL)
		for _, l := range existingLinks {
			if normalizeFriendLinkURL(l.URL) == normalizedNew {
				c.JSON(400, gin.H{"code": 400, "message": "该网址已存在或已在申请中，请勿重复提交"})
				return
			}
		}

		if err := sa.DB.Create(&link).Error; err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "Failed to create friend link"})
			return
		}
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    link,
	})
}
