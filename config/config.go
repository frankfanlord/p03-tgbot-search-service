package config

import (
	"encoding/json"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

type (
	Runtime struct {
		WD   string `yaml:"wd"`
		CPUs uint8  `yaml:"cpus"`
	}

	Build struct {
		Time string `yaml:"time"`
		Hash string `yaml:"hash"`
	}

	Log struct {
		Filter uint8  `yaml:"filter"`
		Caller bool   `yaml:"caller"`
		Output string `yaml:"output"`
	}

	MySQL struct {
		DSN string `yaml:"dsn"`
	}

	Elasticsearch struct {
		Address  []string `yaml:"address"`
		Username string   `yaml:"username"`
		Password string   `yaml:"password"`
	}

	Nats struct {
		Address  []string `yaml:"address"`
		Username string   `yaml:"username"`
		Password string   `yaml:"password"`
		Token    string   `yaml:"token"`
	}

	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       uint8  `yaml:"db"`
	}

	Web struct {
		Prefix  string `yaml:"prefix"`
		Address string `yaml:"address"`
	}

	Configuration struct {
		Ident         string        `yaml:"ident"`
		PodID         string        `yaml:"pod_id"`
		Log           Log           `yaml:"log"`
		MySQL         MySQL         `yaml:"mysql"`
		Elasticsearch Elasticsearch `yaml:"elasticsearch"`
		Nats          Nats          `yaml:"nats"`
		Redis         Redis         `yaml:"redis"`
		Web           Web           `yaml:"web"`
		Runtime       Runtime       `yaml:"runtime"`
		Build         Build         `yaml:"build"`
	}
)

var (
	BuildTime = ""
	BuildHash = ""
	_runtime  = Runtime{}
	_config   = Configuration{}
)

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err.Error())
	}

	_runtime.WD = wd
	_runtime.CPUs = uint8(runtime.NumCPU())

	_config.Runtime = _runtime

	_config.Build.Time = BuildTime
	_config.Build.Hash = BuildHash
}

func Instance() Configuration { return _config }

func Init(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &_config)
}

func (c Configuration) String() string {
	data, err := json.Marshal(&c)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
