package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"jarvis/dao/db/elasticsearch"
	"regexp"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

type SearchContent struct {
	Content string `json:content`
	Link    string `json:link`
}

type SearchResponse struct {
	Content  []SearchContent `json:"content"`
	LastSort []float64       `json:"last_sort"`
	Next     bool            `json:"next"`
}

type Result struct {
	Hits struct {
		Hits []struct {
			Source struct {
				Link   string `json:"link"`
				Photos int    `json:"photos"`
				Videos int    `json:"videos"`
				Voices int    `json:"voices"`
				Files  int    `json:"files"`
			} `json:"_source"`
			Highlight struct {
				Content []string `json:"content"`
			} `json:"highlight"`
			Sort []float64 `json:"sort"`
		} `json:"hits"`
	} `json:"hits"`
}

// all group channel text images videos voices files image+videos
func Search(t uint8, text string, sort []float64) (*SearchResponse, error) {
	pit := _pm.Get()
	if pit == "" {
		return nil, errors.New("empty pid")
	}

	// 构建搜索体
	condition := map[string]any{
		"size": 10,
		"sort": []any{
			map[string]any{
				"score": map[string]any{
					"order": "desc",
				},
			},
			map[string]any{
				"videos": map[string]any{
					"order": "desc",
				},
			},
			map[string]any{
				"photos": map[string]any{
					"order": "desc",
				},
			},
		},
		"highlight": map[string]any{
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
			"fields": map[string]any{
				"content": map[string]any{
					"fragment_size":       30,
					"number_of_fragments": 1,
				},
			},
		},
		"pit": map[string]any{
			"id":         pit,
			"keep_alive": "5m",
		},
		"track_total_hits": false,
	}

	if sort != nil && len(sort) > 0 {
		condition["search_after"] = sort
	}

	filter := make([]any, 0)

	switch t {
	case 1: // group
		{
		}
	case 2: // channel
		{
		}
	case 3: // videos
		{
			filter = append(filter, []any{
				map[string]any{
					"range": map[string]any{
						"videos": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{"term": map[string]any{"photos": 0}},
				map[string]any{"term": map[string]any{"voices": 0}},
				map[string]any{"term": map[string]any{"files": 0}},
			}...)
		}
	case 4: // images
		{
			filter = append(filter, []any{
				map[string]any{
					"range": map[string]any{
						"photos": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{"term": map[string]any{"videos": 0}},
				map[string]any{"term": map[string]any{"voices": 0}},
				map[string]any{"term": map[string]any{"files": 0}},
			}...)
		}

	case 5: // voices
		{
			filter = append(filter, []any{
				map[string]any{
					"range": map[string]any{
						"voices": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{"term": map[string]any{"photos": 0}},
				map[string]any{"term": map[string]any{"videos": 0}},
				map[string]any{"term": map[string]any{"files": 0}},
			}...)
		}
	case 6: // text
		{
			filter = append(filter, []any{
				map[string]any{"term": map[string]any{"photos": 0}},
				map[string]any{"term": map[string]any{"videos": 0}},
				map[string]any{"term": map[string]any{"voices": 0}},
				map[string]any{"term": map[string]any{"files": 0}},
			}...)
		}
	case 7: // files
		{
			filter = append(filter, []any{
				map[string]any{
					"range": map[string]any{
						"files": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{"term": map[string]any{"photos": 0}},
				map[string]any{"term": map[string]any{"videos": 0}},
				map[string]any{"term": map[string]any{"voices": 0}},
			}...)
		}
	case 8: // image+video
		{
			filter = append(filter, []any{
				map[string]any{
					"range": map[string]any{
						"photos": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{
					"range": map[string]any{
						"videos": map[string]any{
							"gte": 1,
						},
					},
				},
				map[string]any{"term": map[string]any{"voices": 0}},
				map[string]any{"term": map[string]any{"files": 0}},
			}...)
		}
	default: // all
		{
		}
	}

	query := map[string]any{
		"bool": map[string]any{
			"must": []any{
				map[string]any{
					"match": map[string]any{
						"content": text,
					},
				},
			},
			"filter": filter,
		},
	}

	condition["query"] = query

	// 序列化为 JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(condition); err != nil {
		return nil, err
	}

	// 构建 Search 请求
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(3000))
	defer cancel()
	res, err := elasticsearch.Instance().Search(
		elasticsearch.Instance().Search.WithContext(ctx),
		elasticsearch.Instance().Search.WithBody(&buf),
		elasticsearch.Instance().Search.WithPretty(),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, errors.New(fmt.Sprintf("%s : %s", res.Status(), res.String()))
	}

	data, raErr := io.ReadAll(res.Body)
	if raErr != nil {
		return nil, raErr
	}

	result := new(Result)
	if err = sonic.Unmarshal(data, result); err != nil {
		return nil, err
	}

	response := &SearchResponse{Content: make([]SearchContent, 0), LastSort: make([]float64, 0), Next: len(result.Hits.Hits) >= 10}

	if result.Hits.Hits != nil && len(result.Hits.Hits) != 0 {
		for idx, hit := range result.Hits.Hits {
			content := processHighlight(hit.Highlight.Content[0])
			content = EscapeMarkdownV2(content)

			prefix := "💬"
			if hit.Source.Videos > 0 {
				prefix = "🎬"
			} else if hit.Source.Photos > 0 {
				prefix = "🖼️"
			} else if hit.Source.Voices > 0 {
				prefix = "🎧"
			} else if hit.Source.Files > 0 {
				prefix = "📁"
			}

			response.Content = append(response.Content, SearchContent{
				Content: prefix + content,
				Link:    fmt.Sprintf("https://t.me%s", hit.Source.Link),
			})

			if idx == (len(result.Hits.Hits) - 1) {
				response.LastSort = hit.Sort
			}
		}
	}

	return response, nil
}

// EscapeMarkdownV2 将输入文本转义为MarkdownV2格式
// MarkdownV2是Telegram Bot API使用的Markdown格式
// 需要转义的字符: _ * [ ] ( ) ~ ` > # + - = | { } . !
func EscapeMarkdownV2(text string) string {
	// MarkdownV2中需要转义的特殊字符
	specialChars := []string{
		"_", "*", "[", "]", "(", ")", "~", "`",
		">", "#", "+", "-", "=", "|", "{", "}",
		".", "!",
	}

	result := text

	// 对每个特殊字符进行转义
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}

	return result
}

func processHighlight(content string) string {
	// 找最后一个 <em> 的位置
	lastEmIdx := strings.LastIndex(content, "<em>")
	if lastEmIdx == -1 {
		// 没有高亮，直接清洗并返回前20字符
		cleaned := cleanContent(content)
		return substringByRune(cleaned, 0, 20)
	}

	// 切成 prefix 和 suffix
	prefix := content[:lastEmIdx]
	suffix := content[lastEmIdx:]

	// 清洗两个部分
	prefixClean := cleanContent(prefix)
	suffixClean := cleanContent(suffix)

	// 转为 []rune
	suffixRunes := []rune(suffixClean)
	if len(suffixRunes) >= 25 {
		return string(suffixRunes[:25])
	}

	// 不够就从 prefix 的尾部补
	needed := 25 - len(suffixRunes)
	prefixRunes := []rune(prefixClean)
	if needed > len(prefixRunes) {
		needed = len(prefixRunes)
	}
	resultRunes := append(prefixRunes[len(prefixRunes)-needed:], suffixRunes...)

	return string(resultRunes)
}

func cleanContent(s string) string {
	s = strings.ReplaceAll(s, "<em>", "")
	s = strings.ReplaceAll(s, "</em>", "")
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "，", "")
	s = strings.ReplaceAll(s, ":", "")
	spaceRegex := regexp.MustCompile(`\s+`)
	s = spaceRegex.ReplaceAllString(s, "")
	return s
}

// 获取子串（按 rune 位置截取）
func substringByRune(s string, start, length int) string {
	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	end := start + length
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}
