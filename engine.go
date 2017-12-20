package siem

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"github.com/devplayg/golibs/crypto"
	"github.com/devplayg/golibs/orm"
	log "github.com/sirupsen/logrus"
	"runtime"
	"crypto/sha256"
)

var (
	CmdFlags *flag.FlagSet
	encKey   []byte
)

func init() {
	CmdFlags = flag.NewFlagSet("", flag.ExitOnError)
	key := sha256.Sum256([]byte("D?83F4 E?E"))
	encKey = key[:]
}

type Engine struct {
	ConfigPath  string
	Config      map[string]string
	Interval    int64
	debug       bool
	cpuCount    int
	processName string
	logOutput   int // 0: STDOUT, 1: File
}

func NewEngine(debug bool, cpuCount int, interval int64) *Engine {
	e := Engine{
		processName: strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])),
		cpuCount:    cpuCount,
		debug:       debug,
		Interval:    interval,
	}
	e.ConfigPath = filepath.Join(filepath.Dir(os.Args[0]), e.processName+".enc")
	e.initLogger()
	return &e
}

func (e *Engine) Start() error {
	config, err := e.getConfig()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	if _, ok := config["db.hostname"]; !ok {
		return errors.New("Invalid configurations")
	}

	err = e.initDatabase()
	if err != nil {
		return err
	}

	runtime.GOMAXPROCS(e.cpuCount)
	log.Debugf("GOMAXPROCS set to %d", runtime.GOMAXPROCS(0))
	return nil
}

func (e *Engine) initLogger() error {

	// Set log format
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		DisableColors: true,
	})

	// Set log file
	logFile := filepath.Join(filepath.Dir(os.Args[0]), e.processName+".log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err == nil {
		log.SetOutput(file)
		e.logOutput = 1
		//fmt.Printf("Output: %s\n", file.Name())
	} else {
		//		log.Error("Failed to log to file, using default stderr")
		e.logOutput = 0
		log.SetOutput(os.Stdout)
	}

	// Set log level
	if e.debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if log.GetLevel() != log.InfoLevel {
		log.Infof("LoggingLevel=%s", log.GetLevel())
	}

	return nil
}

func (e *Engine) initDatabase() error {
	connStr := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?allowAllFiles=true&charset=utf8&parseTime=true&loc=%s",
		e.Config["db.username"],
		e.Config["db.password"],
		e.Config["db.hostname"],
		e.Config["db.port"],
		e.Config["db.database"],
		"Asia%2FSeoul")
	log.Debugf("Database connection string: %s", connStr)
	err := orm.RegisterDataBase("default", "mysql", connStr, 3, 3)
	return err
}

func WaitForSignals() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signalCh:
		log.Println("Signal received, shutting down...")
	}
}

func (e *Engine) getConfig() (map[string]string, error) {
	if _, err := os.Stat(e.ConfigPath); os.IsNotExist(err) {
		return nil, errors.New("Configuration file not found. Use '-config' option.")
	} else {
		config := make(map[string]string)
		err := crypto.LoadEncryptedObjectFile(e.ConfigPath, encKey, &config)
		return config, err
	}
}

func (e *Engine) SetConfig(extra string) error {
	config, err := e.getConfig()
	if config == nil {
		config = make(map[string]string)
	}

	fmt.Println("Setting configuration")
	e.readInput("db.hostname", config)
	e.readInput("db.port", config)
	e.readInput("db.username", config)
	e.readInput("db.password", config)
	e.readInput("db.database", config)

	if len(extra) > 0 {
		arr := strings.Split(extra, ",")
		for _, k := range arr {
			e.readInput(k, config)
		}
	}
	err = crypto.SaveObjectToEncryptedFile(e.ConfigPath, encKey, config)
	if err == nil {
		fmt.Println("Done")
	} else {
		fmt.Println(err.Error())
	}

	return err
}

func (e *Engine) readInput(key string, config map[string]string) {
	if val, ok := config[key]; ok && len(val) > 0 {
		fmt.Printf("%-16s = (%s) ", key, val)
	} else {
		fmt.Printf("%-16s = ", key)
	}

	reader := bufio.NewReader(os.Stdin)
	newVal, _ := reader.ReadString('\n')
	newVal = strings.TrimSpace(newVal)
	if len(newVal) > 0 {
		config[key] = newVal
	}
}

func PrintHelp() {
	fmt.Println(strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])))
	CmdFlags.PrintDefaults()
}

func DisplayVersion(prodName, version string) {
	fmt.Printf("%s, v%s\n", prodName, version)
}

