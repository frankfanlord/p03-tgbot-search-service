package core

import (
	"net/http"
	"search-service/core/search"

	"github.com/gin-gonic/gin"
)

type SearchRequest struct {
	Type  uint8     `json:"type"`  // 0:all 1:groupt 2:channel 3:text 4:photo 5:video 6:voice 7:file 8:photo/video
	Words string    `json:"words"` // 搜索关键词
	Sort  []float64 `json:"sort"`
}

func doSearch(ctx *gin.Context) {
	// 获取 Header 参数
	uid := ctx.GetHeader("user-id")

	if uid == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing user-id header"})
		return
	}

	// 获取 JSON Body 参数
	request := new(SearchRequest)
	if err := ctx.ShouldBindJSON(request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	response, err := search.Search(request.Type, request.Words, request.Sort)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	// 返回所有参数信息
	ctx.JSON(http.StatusOK, response)
}
