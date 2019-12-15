package config

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	DB         string `json:"db"`
	Port       int    `json:"port"`
	Socket     string `json:"socket"`
	NoReg      bool   `json:"noreg"`
	Debug      bool   `json:"debug"`
	Foreground bool   `json:"foreground"`
	UseCORS    bool   `json:"cors"`
}

var Conf = Config{
	DB:         "sf.db",
	Port:       8888,
	Debug:      false,
	NoReg:      false,
	Foreground: false,
	UseCORS:    false,
}

func InitConf(path string) (err error) {
	if data, ierr := ioutil.ReadFile(path); ierr != nil {
		err = ierr
		return
	} else if ierr = json.Unmarshal(data, &Conf); ierr != nil {
		err = ierr
		return
	}
	return
}

var MiscData = struct {
	Version      string
	LoadedConfig string
}{}
