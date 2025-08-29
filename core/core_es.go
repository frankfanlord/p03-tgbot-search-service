package core

import (
	"errors"
	"jarvis/dao/db/elasticsearch"
	"jarvis/logger"
	"strings"
)

func initESMapping() error {
	// 1. cog
	if err := initMapping("cog", _cogMap); err != nil {
		return err
	}

	logger.App().Infoln("=================== index cog ===================")

	// 2. message
	if err := initMapping("message", _messageMap); err != nil {
		return err
	}

	logger.App().Infoln("=================== index message ===================")

	return nil
}

func initMapping(indexName, mapping string) error {
	eRsp, eErr := elasticsearch.Instance().Indices.Exists([]string{indexName})
	if eErr != nil {
		return eErr
	}
	defer func() { _ = eRsp.Body.Close() }()

	if eRsp.StatusCode == 404 {
		cRsp, cErr := elasticsearch.Instance().Indices.Create(indexName, elasticsearch.Instance().Indices.Create.WithBody(strings.NewReader(mapping)))
		if cErr != nil {
			return cErr
		}
		defer func() { _ = cRsp.Body.Close() }()

		if cRsp.IsError() {
			return errors.New(cRsp.String())
		}

		return nil
	} else if eRsp.StatusCode == 200 {
		return nil
	}

	return errors.New(eRsp.String())
}

// number_of_shards make it more to 100
const _messageMap = `{
  "settings": {
    "number_of_shards": 1,  
    "number_of_replicas": 1,  
    "index.queries.cache.enabled": true 
  },
  "mappings": {
    "properties": {
      "id": {
        "type": "keyword"
      },
      "content": {
        "type": "text",
        "analyzer": "ik_max_word",      
        "search_analyzer": "ik_smart",   
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256         
          }
        }
      },
      "link": {
        "type": "keyword"
      },
      "score": {
        "type": "integer"                 
      },
      "photos": {
        "type": "integer"               
      },
      "videos": {
        "type": "integer"               
      },
      "voices": {
        "type": "integer"               
      },
      "files": {
        "type": "integer"               
      }
    }
  }
}`

const _cogMap = `{
  "settings": {
    "number_of_shards": 1,  
    "number_of_replicas": 1  
  },
  "mappings": {
    "properties": {
      "id": {
        "type": "keyword"
      },
      "title": {
        "type": "text",
        "analyzer": "ik_max_word",      
        "search_analyzer": "ik_smart",   
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256         
          }
        }
      },
      "members": {
        "type": "integer"               
      },
      "messages": {
        "type": "integer"               
      }
    }
  }
}`
