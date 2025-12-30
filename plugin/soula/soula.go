package soula

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"os"
	"pansou/model"
	"pansou/plugin"
	"path/filepath"
	"sync"
)

type SoulaPlugin struct {
	*plugin.BaseAsyncPlugin

	mu          sync.RWMutex
	initialized bool // 初始化状态标记
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

	fmt.Printf("[SOULA] Web路由已注册: /api/:param\n")
}

func (sa *SoulaPlugin) handleCategories(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"categories": []gin.H{
				{
					"name": "\u70ed\u641c",
					"items": []gin.H{
						{
							"term":  "\u5510\u671d\u8be1\u4e8b\u5f55",
							"score": 2497,
						},
						{
							"term":  "\u6ce2\u591a\u91ce\u7ed3\u8863",
							"score": 1473,
						},
						{
							"term":  "\u73b0\u5728\u5c31\u51fa\u53d1",
							"score": 1393,
						},
						{
							"term":  "\u51e1\u4eba\u4fee\u4ed9",
							"score": 1391,
						},
					},
				},
			}},
	})
}

func (sa *SoulaPlugin) handleResourcesRandom(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": []gin.H{
				{
					"id":             405,
					"unique_id":      "FLMdongtianfudi_15549",
					"channel":        "FLMdongtianfudi",
					"ai_title":       "\u4f0d\u51cc\u67ab\u5409\u4ed6\u8bad\u7ec3\u8425\uff1a\u7436\u97f3\u901f\u6210\uff0c\u524d\u536b\u7f16\u914d\uff0c\u52a9\u4f60\u5f39\u594f\u66f4\u51fa\u5f69\uff01",
					"ai_description": "\u8fd8\u5728\u4e3a\u5409\u4ed6\u7436\u97f3\u53d1\u6101\u5417\uff1f\u60f3\u8ba9\u4f60\u7684\u5409\u4ed6\u6f14\u594f\u66f4\u6709\u4e2a\u6027\uff0c\u7f16\u66f2\u66f4\u5177\u521b\u610f\uff1f\u4f0d\u51cc\u67ab\u8001\u5e08\u7684\u8bad\u7ec3\u8425\u6765\u5566\uff01\u8fd9\u91cc\u4e0d\u4ec5\u6709\u7cfb\u7edf\u7684\u7436\u97f3\u8bad\u7ec3\uff0c\u8ba9\u4f60\u6307\u5c16\u6d41\u7545\uff0c\u66f4\u80fd\u6df1\u5165\u5b66\u4e60\u524d\u536b\u5409\u4ed6\u7f16\u914d\u4e0e\u521b\u4f5c\uff0c\u624b\u628a\u624b\u6559\u4f60\u5982\u4f55\u5c06\u8111\u6d77\u4e2d\u7684\u65cb\u5f8b\u53d8\u6210\u52a8\u542c\u7684\u4e50\u7ae0\u3002\u4ece\u57fa\u672c\u529f\u5230\u98ce\u683c\u5851\u9020\uff0c\u52a9\u4f60\u5f7b\u5e95\u7a81\u7834\u74f6\u9888\uff0c\u8ba9\u4f60\u7684\u5409\u4ed6\u6c34\u5e73\u98de\u901f\u63d0\u5347\uff0c\u5f39\u594f\u51fa\u4e13\u5c5e\u4e8e\u4f60\u7684\u72ec\u7279\u97f3\u4e50\u98ce\u683c\uff01",
					"ai_tags": []string{
						"\u4f0d\u51cc\u67ab",
						"\u5409\u4ed6\u6559\u5b66",
						"\u7436\u97f3\u7ec3\u4e60",
						"\u7f16\u66f2\u6280\u5de7",
						"\u4e50\u5668\u5b66\u4e60",
						"\u97f3\u4e50\u63d0\u5347",
						"\u5409\u4ed6\u521b\u4f5c",
					},
					"image_urls": []string{
						"https://image.jkai.de/2025/12/b98eae0c320edf413bc31a67874161d9.jpg",
					},
					"original_title":   "\u4f0d\u51cc\u67ab\u7436\u97f3\u8bad\u7ec3\u8425+\u524d\u536b\u5409\u4ed6\u7f16\u914d\u4e0e\u521b\u4f5c\uff0c\u97f3\u4e50\u6280\u5de7+\u521b\u4f5c\u65b9\u6cd5+\u6f14\u594f\u63d0\u5347",
					"original_content": "\u4f0d\u51cc\u67ab\u7436\u97f3\u8bad\u7ec3\u8425+\u524d\u536b\u5409\u4ed6\u7f16\u914d\u4e0e\u521b\u4f5c\uff0c\u97f3\u4e50\u6280\u5de7+\u521b\u4f5c\u65b9\u6cd5+\u6f14\u594f\u63d0\u5347\u7ed3\u5408\u7436\u97f3\u8bad\u7ec3\u4e0e\u5409\u4ed6\u7f16\u914d\uff0c\u63d0\u4f9b\u5b9e\u9645\u7684\u6280\u5de7\u4e0e\u521b\u4f5c\u6307\u5357\uff0c\u5e2e\u52a9\u5409\u4ed6\u7231\u597d\u8005\u5728\u6f14\u594f\u4e0e\u4f5c\u66f2\u4e0a\u5b9e\u73b0\u7a81\u7834\uff0c\u63d0\u5347\u97f3\u4e50\u8868\u73b0\u529b\u4e0e\u4e2a\u6027\u98ce\u683c\u3002",
					"my_pan_url":       "https://pan.quark.cn/s/b9a1c92e1936",
					"my_pan_password":  "",
					"views":            714,
					"status":           1,
					"status_text":      "\u5df2\u5b8c\u6210",
					"category":         "\u89c6\u9891",
					"created_at":       "2025-12-25 10:00:02",
				},
			},
		},
	})
}

func (sa *SoulaPlugin) handleResource(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": 1,
			"merged_by_type": gin.H{
				"aliyun": []gin.H{
					{
						"url":      "https://www.alipan.com/s/B6t4whnnVQz",
						"password": "",
						"note":     "【学堂在线】算法设计与分析 - 清华大学",
						"datetime": "2025-12-30T14:00:59Z",
						"source":   "tg:shareAliyun",
						"images": []string{
							"https://cdn5.telesco.pe/file/lgM3olc5_0X9492Fk-dvoWRiEsiikO4KbEtPl4Wiu8BEArHckdUUBW2wDrqiQ4KBhqWOsX2r_3ar3fNkvShZAVDBZayPM1CYw62zVZjkvbKQcf3_qt9eSpOMtJwMdMclnrrdfXX5VgAgnE0kP_wNtIVM5szVhIhUMbsnShjh613cwTakHtNvQi9TnIC9Dd731voaBRI5F4RMxCuMwirKMEWMTzKKDmixVlEIYW_Wr3kp070SIzwvPyEcCNkdTRG5A77wRJyaiG7OxRGiTmUZMzz9W_S4RJt1IsCfwT4kmPxBfu8X9mouRlt9KXT6-WjH0yIWzchNBwFEfiKz3ZpNuQ.jpg",
						},
					},
				},
			},
		},
	})
}

func (sa *SoulaPlugin) handleResources(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items":        []gin.H{},
			"total":        0,
			"current_page": 1,
			"last_page":    1,
			"per_page":     10,
		},
	})
}
