package main

import (
	"chat-room-go/api/router"
	"chat-room-go/config"
	"chat-room-go/model/redis"
	"chat-room-go/model/redis_read"
	"chat-room-go/util"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var cfgPath = "config/conf/config.yaml"

type RedisConnect struct {
	host       []string
	masterName string
	password   string
}

func main() {

	err := config.InitConfig(cfgPath)
	if err != nil {
		glog.Error(err)
		panic(err)
	}

	// set gin run mode
	gin.SetMode(viper.GetString("runmode"))

	// init redis sentinel client
	ipAddressFile, _ := os.Stat("./ip-address") // 之前已经启动过
	if ipAddressFile != nil {
		// 连接还存活的redis集群，目的是启动集群，通过获取master节点ip地址
		rc, _ := ReadRedisConfig()
		err := redis.InitRedisSentinel(rc.host, rc.masterName, rc.password)
		if err != nil {
		}
		// 延迟2s
		time.Sleep(time.Second * 2)
		// 获取master节点地址  ip:端口
		masterIP := redis.GetRedisMasterIP()
		// 去除端口号
		masterIP = strings.Split(masterIP, ":")[0]

		// 机器完全宕机后，重新生成集群
		if masterIP == "" {
			hosts := getRedisClusterHosts()
			masterIP = strings.Split(hosts[0], ":")[0]
		}

		// 启动集群和哨兵
		startSlaveRedisClusterAndSentinel(masterIP)

		// 延迟2s
		time.Sleep(time.Second * 2)

		// 初始化
		redis_read.InitRedis()
		redis.InitRedisSentinel(rc.host, rc.masterName, rc.password)

	}

	// init router and middleware
	r := router.Load(gin.New())

	// run gin service
	if err := r.Run(viper.GetString("url")); err != nil {
		glog.Info(err)
	}
}

func sendQuickMessage() {
	// 服务终止
	masterHost := util.ReadWithFile("./master-address")
	localIP := util.GetIp()
	masterIP := strings.Split(masterHost, ":")[0]

	// 死掉的机器不是自己
	if masterIP != localIP {
		return
	}
	// 死掉的机器是自己，给其他机器发送信号
	otherHostsString := util.ReadWithFile("./ip-address")
	hosts := strings.Split(otherHostsString, "\n")
	for i := 0; i < len(hosts); i++ {
		ipAddr := strings.Split(hosts[i], ":")[0]
		if ipAddr != localIP {
			_, err := http.Get(fmt.Sprintf("http://%s:8080/startCluster", ipAddr))
			if err != nil {
				log.Println(err)
			}
		}
	}

}

// ReadRedisConfig 读取Redis配置
func ReadRedisConfig() (RedisConnect, error) {
	var rc RedisConnect

	hosts := getRedisClusterHosts()
	if len(hosts) == 3 {
		masterName := viper.GetString("redis-sentinel.master_name")
		password := viper.GetString("redis-sentinel.password")
		rc.host = hosts
		rc.masterName = masterName
		rc.password = password
	} else {
		return rc, errors.New("host err")
	}
	return rc, nil
}

func getRedisClusterHosts() []string {
	hostString := util.ReadWithFile("./ip-address")
	hosts := strings.Split(hostString, "\n")
	return hosts
}

func startSlaveRedisClusterAndSentinel(masterIP string) {
	util.ExecShell(fmt.Sprintf("sudo sh config/script/rediscluster/redismasl.sh %s %s", "slave", masterIP))
	util.ExecShell(fmt.Sprintf("sudo sh config/script/rediscluster/sentinel.sh %s", masterIP))
}

//func startMasterRedisClusterAndSentinel() {
//	util.ExecShell(fmt.Sprintf("sudo sh config/script/rediscluster/redismasl.sh %s %s", "slave", masterIP))
//	util.ExecShell(fmt.Sprintf("sudo sh config/script/rediscluster/sentinel.sh %s", masterIP))
//}
