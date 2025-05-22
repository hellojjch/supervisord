package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/ochinchina/go-ini"
	log "github.com/sirupsen/logrus"
)

// ConfigProvider 接口定义了配置提供者的行为
type ConfigProvider interface {
	// 获取配置内容
	GetConfig() (*ini.Ini, error)
	// 保存配置内容
	SaveConfig(content string) error
	// 获取配置目录
	GetConfigDir() string
}

// FileConfigProvider 本地文件配置提供者
type FileConfigProvider struct {
	configFile string
}

// NewFileConfigProvider 创建一个新的本地文件配置提供者
func NewFileConfigProvider(configFile string) *FileConfigProvider {
	return &FileConfigProvider{configFile: configFile}
}

// GetConfig 从本地文件获取配置
func (p *FileConfigProvider) GetConfig() (*ini.Ini, error) {
	myini := ini.NewIni()
	myini.LoadFile(p.configFile)
	return myini, nil
}

// SaveConfig 保存配置到本地文件
func (p *FileConfigProvider) SaveConfig(content string) error {
	return ioutil.WriteFile(p.configFile, []byte(content), 0644)
}

// GetConfigDir 获取配置文件所在目录
func (p *FileConfigProvider) GetConfigDir() string {
	return filepath.Dir(p.configFile)
}

// NacosConfigProvider Nacos配置提供者
type NacosConfigProvider struct {
	client      config_client.IConfigClient
	dataId      string
	group       string
	namespace   string
	configCache string
	configDir   string
	mutex       sync.RWMutex
}

// NacosConfig Nacos配置
type NacosConfig struct {
	ServerAddr  string `json:"serverAddr"`  // 服务器地址，如 127.0.0.1:8848
	Namespace   string `json:"namespace"`   // 命名空间ID
	Group       string `json:"group"`       // 配置分组
	DataId      string `json:"dataId"`      // 配置ID
	Username    string `json:"username"`    // 用户名
	Password    string `json:"password"`    // 密码
	NotUseCache bool   `json:"not_use_cache"` // 是否不使用缓存
}

// NewNacosConfigProvider 创建一个新的Nacos配置提供者
func NewNacosConfigProvider(config NacosConfig) (*NacosConfigProvider, error) {
	// 设置默认值
	if config.Group == "" {
		config.Group = "DEFAULT_GROUP"
	}

	// 创建clientConfig
	clientConfig := constant.ClientConfig{
		NamespaceId:         config.Namespace,
		TimeoutMs:           10000, // 增加超时时间
		NotLoadCacheAtStart: config.NotUseCache, // 根据配置决定是否使用缓存
		LogDir:              "logs",
		CacheDir:            "cache",
		LogLevel:            "debug", // 设置为debug级别以便排查问题
		Username:            config.Username,
		Password:            config.Password,
	}

	// 解析服务器地址
	parts := strings.Split(config.ServerAddr, ":")
	port := uint64(8848)
	if len(parts) > 1 {
		if p, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
			port = p
		}
	}

	// 创建serverConfig
	serverConfigs := []constant.ServerConfig{
		{
			IpAddr:      parts[0],
			Port:        port,
			ContextPath: "/nacos", // 添加上下文路径
			Scheme:      "http",   // 指定协议
		},
	}

	// 确保日志和缓存目录存在
	os.MkdirAll("logs", 0755)
	os.MkdirAll("cache", 0755)

	// 创建动态配置客户端
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("创建Nacos客户端失败: %v", err)
	}

	// 创建临时目录用于存储配置
	tempDir, err := ioutil.TempDir("", "supervisord-nacos-")
	if err != nil {
		return nil, err
	}

	provider := &NacosConfigProvider{
		client:    client,
		dataId:    config.DataId,
		group:     config.Group,
		namespace: config.Namespace,
		configDir: tempDir,
	}

	// 初始加载配置
	_, err = provider.GetConfig()
	if err != nil {
		return nil, err
	}

	// 监听配置变更
	provider.listenConfig()

	return provider, nil
}

// 解析端口号
func parsePort(addr string) uint64 {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return 8848 // 默认端口
	}

	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	if port <= 0 {
		return 8848
	}
	return uint64(port)
}

// GetConfig 从Nacos获取配置
func (p *NacosConfigProvider) GetConfig() (*ini.Ini, error) {
	p.mutex.RLock()
	cachedConfig := p.configCache
	p.mutex.RUnlock()

	var content string
	var err error
	maxRetries := 3

	// 如果缓存为空，则从Nacos获取
	if cachedConfig == "" {
		for i := 0; i < maxRetries; i++ {
			content, err = p.client.GetConfig(vo.ConfigParam{
				DataId: p.dataId,
				Group:  p.group,
			})

			if err == nil && content != "" {
				break
			}

			log.Warnf("尝试获取Nacos配置失败(%d/%d): %v", i+1, maxRetries, err)
			time.Sleep(time.Second * time.Duration(i+1))
		}

		if err != nil {
			return nil, fmt.Errorf("从Nacos获取配置失败: %v", err)
		}

		if content == "" {
			return nil, fmt.Errorf("获取到的配置内容为空")
		}

		p.mutex.Lock()
		p.configCache = content
		p.mutex.Unlock()

		cachedConfig = content
	}

	// 验证并解析配置
	myini := ini.NewIni()
	myini.Load([]byte(cachedConfig))
	return myini, nil
}

// SaveConfig 保存配置到Nacos
func (p *NacosConfigProvider) SaveConfig(content string) error {
	// 发布配置
	success, err := p.client.PublishConfig(vo.ConfigParam{
		DataId:  p.dataId,
		Group:   p.group,
		Content: content,
	})

	if err != nil {
		return err
	}

	if !success {
		return fmt.Errorf("发布配置失败")
	}

	// 更新缓存
	p.mutex.Lock()
	p.configCache = content
	p.mutex.Unlock()

	return nil
}

// GetConfigDir 获取配置目录
func (p *NacosConfigProvider) GetConfigDir() string {
	return p.configDir
}

// 监听配置变更
func (p *NacosConfigProvider) listenConfig() {
	p.client.ListenConfig(vo.ConfigParam{
		DataId: p.dataId,
		Group:  p.group,
		OnChange: func(namespace, group, dataId, data string) {
			log.Infof("Nacos配置发生变更: %s/%s/%s", namespace, group, dataId)
			p.mutex.Lock()
			p.configCache = data
			p.mutex.Unlock()
		},
	})
}

func validateNacosConfig(config NacosConfig) error {
	if config.ServerAddr == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	if config.DataId == "" {
		return fmt.Errorf("DataId不能为空")
	}

	// 验证服务器地址格式
	parts := strings.Split(config.ServerAddr, ":")
	if len(parts) != 2 {
		return fmt.Errorf("服务器地址格式不正确，应为 IP:PORT")
	}

	return nil
}
