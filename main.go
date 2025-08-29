package main

import (
	"flag"
	"fmt"
	"jarvis/dao/db/elasticsearch"
	"jarvis/dao/db/mysql"
	"jarvis/dao/db/redis"
	"jarvis/logger"
	"jarvis/middleware/mq/nats"
	"os"
	"os/signal"
	"path/filepath"
	"search-service/config"
	"search-service/core"
	"syscall"

	"github.com/duke-git/lancet/v2/fileutil"
)

var (
	cfd = flag.String("cfd", "config", "dir of configurations")
	cff = flag.String("cff", "default.yaml", "configuration file")
)

func init() {
	flag.Parse()

	if err := initConfig(); err != nil {
		panic(fmt.Sprintf("failed to initialize config: %s", err.Error()))
	}

	if err := initLogger(); err != nil {
		panic(fmt.Sprintf("failed to initialize config: %s", err.Error()))
	}

	if err := initDatabase(); err != nil {
		panic(fmt.Sprintf("failed to initialize database: %s", err.Error()))
	}

	if err := initCache(); err != nil {
		panic(fmt.Sprintf("failed to initialize cache: %s", err.Error()))
	}

	if err := initService(); err != nil {
		panic(fmt.Sprintf("failed to initialize service: %s", err.Error()))
	}

	logger.System().Infof("Ident [%s][%s] process configuration : %s", config.Instance().Ident, config.Instance().PodID, config.Instance().String())
}

func main() {
	logger.System().Infof("Ident [%s] process start", config.Instance().Ident)
	defer logger.System().Infof("Ident [%s] process stoped", config.Instance().Ident)

	if err := core.Start(); err != nil {
		logger.App().Panicf("failed to start core-service: %s", err.Error())
	}

	waitForShutdown()
}

func waitForShutdown() {
	sg := make(chan os.Signal, 1)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM)

	select {
	case s := <-sg:
		{
			logger.System().Infof("Got signal %s, shutting down... ", s.String())

			if err := core.Shutdown(); err != nil {
				logger.System().Errorf("failed to shutdown server: %s", err.Error())
			}
		}
	}
}

// =====================================================================================================================

func initConfig() error {
	configPath := filepath.Join(*cfd, *cff)

	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(config.Instance().Runtime.WD, configPath)
	}

	if !fileutil.IsExist(configPath) {
		return fmt.Errorf("config file [%s] does not exist", configPath)
	}

	if fileutil.IsDir(configPath) {
		return fmt.Errorf("config file [%s] is a directory", configPath)
	}

	return config.Init(configPath)
}

func initLogger() error {
	outputDir := config.Instance().Log.Output
	if outputDir == "" {
		outputDir = "output"
	}

	outputDir = filepath.Join(config.Instance().Runtime.WD, outputDir)

	if !fileutil.IsExist(outputDir) {
		if err := fileutil.CreateDir(outputDir); err != nil {
			return err
		}
	}

	fileHooker, err := logger.NewFileHook(filepath.Join(outputDir, config.Instance().Ident))
	if err != nil {
		return err
	}

	logger.Init(
		new(logger.DEBUGFormatter),
		config.Instance().Log.Caller,
		logger.Level(config.Instance().Log.Filter),
		fileHooker,
	)

	return nil
}

func initDatabase() error {
	if err := mysql.Init(
		config.Instance().MySQL.DSN,
	); err != nil {
		return err
	}

	if err := elasticsearch.Init(
		config.Instance().Elasticsearch.Address[:],
		config.Instance().Elasticsearch.Username,
		config.Instance().Elasticsearch.Password,
	); err != nil {
		return err
	}

	if err := redis.Init(
		config.Instance().Redis.Address,
		config.Instance().Redis.Password,
		config.Instance().Redis.DB,
	); err != nil {
		return err
	}

	return nil
}

func initCache() error {
	if err := core.LoadCache(); err != nil {
		return err
	}

	return nil
}

func initService() error {
	if err := nats.Init(
		config.Instance().Nats.Username,
		config.Instance().Nats.Password,
		config.Instance().Nats.Token,
		config.Instance().Nats.Address...,
	); err != nil {
		return err
	}

	if err := core.Init(
		config.Instance().Web.Prefix,
		config.Instance().Web.Address,
	); err != nil {
		return err
	}

	return nil
}
